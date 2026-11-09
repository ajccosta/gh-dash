package tui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// Batch merge: with PRs selected (* or a modifier-click on a row), m and the
// Merge button ask once how to merge them all, then merge them one at a time,
// a PR before those stacked on it. Each one starts when the previous one is
// done, so a stacked PR is moved onto the base (Delete branch) before it is
// merged; one GitHub refuses is tried again a few times, as GitHub may still
// be updating it after the previous merge. The batch stops at the first PR
// that does not merge, leaving it and the rest selected.

const (
	batchRetries    = 3
	batchRetryAfter = 5 * time.Second
)

type batchMerge struct {
	sid     tasks.SectionIdentifier
	queue   []*prrow.Data // still to merge; the first one is merging
	method  string
	delete  bool
	retries int // of queue[0]
	merged  int
}

type batchRetryMsg struct{}

// markedSection is the current section if it is a PR section.
func (m *Model) markedSection() *prssection.Model {
	if m.ctx.View != config.PRsView {
		return nil
	}
	s, _ := m.getCurrSection().(*prssection.Model)
	return s
}

// markedPRs is the PRs selected in the current section, nil if none is.
func (m *Model) markedPRs() []*prrow.Data {
	if s := m.markedSection(); s != nil {
		return s.Marked()
	}
	return nil
}

// mergeOrder puts every PR after the one its branch is based on, keeping list
// order otherwise.
func mergeOrder(prs []*prrow.Data) []*prrow.Data {
	left := slices.Clone(prs)
	var order []*prrow.Data
	for len(left) > 0 {
		i := slices.IndexFunc(left, func(p *prrow.Data) bool {
			return stackedOn(p, left) == nil
		})
		if i < 0 {
			i = 0 // a cycle
		}
		order = append(order, left[i])
		left = slices.Delete(left, i, i+1)
	}
	return order
}

// stackedOn is the PR of prs whose branch p is based on.
func stackedOn(p *prrow.Data, prs []*prrow.Data) *prrow.Data {
	for _, q := range prs {
		if q != p && q.Primary.Repository.NameWithOwner == p.Primary.Repository.NameWithOwner &&
			q.Primary.HeadRefName == p.Primary.BaseRefName {
			return q
		}
	}
	return nil
}

func (m *Model) openBatchMergeDialog(sid tasks.SectionIdentifier, prs []*prrow.Data) tea.Cmd {
	if m.batch != nil {
		return m.notifyErr(fmt.Sprintf("Still merging a batch: %d PRs to go", len(m.batch.queue)))
	}
	prs = slices.DeleteFunc(slices.Clone(prs), func(p *prrow.Data) bool {
		return p.Primary.State != "OPEN"
	})
	if len(prs) == 0 {
		return m.notifyErr("None of the selected PRs is open")
	}
	prs = mergeOrder(prs)
	text := []string{"One at a time, in this order; stops at the first that fails:"}
	var stacked []string
	for _, p := range prs {
		text = append(text, fmt.Sprintf("  #%d %s", p.Primary.Number, p.Primary.Title))
		if stackedOn(p, prs) != nil {
			stacked = append(stacked, fmt.Sprintf("#%d", p.Primary.Number))
		}
	}
	if len(stacked) > 0 {
		text = append(text, "",
			fmt.Sprintf("Stacked: %v. With Delete branch yes each is moved onto", stacked),
			"the base first; with no, it merges into its parent's branch.")
	}
	m.dialog.Open(dialog.Spec{
		Title: fmt.Sprintf("Merge %d PRs?", len(prs)),
		Text:  text,
		Fields: []dialog.Field{
			{Name: "method", Title: "Method", Options: []string{"merge", "squash", "rebase"}},
			{Name: "delete", Title: "Delete branch", Options: []string{"no", "yes"}},
		},
		Buttons:  []string{"Merge", "Cancel"},
		Remember: "merge",
		Submit: func(v map[string]string, _ string) tea.Cmd {
			m.batch = &batchMerge{sid: sid, queue: prs, method: v["method"], delete: v["delete"] == "yes"}
			return m.batchMergeNext()
		},
	})
	return nil
}

// batchMergeNext starts merging the first PR of the queue.
func (m *Model) batchMergeNext() tea.Cmd {
	b := m.batch
	if b == nil || len(b.queue) == 0 {
		return nil
	}
	if b.delete {
		return m.planDeleteBranch(b.sid, b.queue[0], b.method)
	}
	return tasks.MergePR(m.ctx, b.sid, b.queue[0], b.method, nil)
}

// batchMergeStep goes on with the batch when the merge of its current PR is
// done.
func (m *Model) batchMergeStep(msg constants.TaskFinishedMsg) tea.Cmd {
	b := m.batch
	if b == nil || len(b.queue) == 0 || msg.SectionId != b.sid.Id || msg.SectionType != b.sid.Type ||
		msg.TaskId != tasks.MergeTaskId(b.queue[0].Primary.Number) {
		return nil
	}
	pr := b.queue[0]
	upd, _ := msg.Msg.(tasks.UpdatePRMsg)
	merged := upd.IsMerged != nil && *upd.IsMerged
	if msg.Err != nil {
		if !merged && b.retries < batchRetries {
			b.retries++
			return tea.Tick(batchRetryAfter, func(time.Time) tea.Msg { return batchRetryMsg{} })
		}
		m.batch = nil
		return m.notifyErr(fmt.Sprintf("Batch merge stopped at #%d: %d merged, %d left selected",
			pr.Primary.Number, b.merged, len(b.queue)))
	}
	if b.sid.Type == prssection.SectionType && b.sid.Id < len(m.prs) {
		s, _ := m.prs[b.sid.Id].(*prssection.Model)
		if s != nil {
			s.Unmark(pr)
		}
	}
	b.queue, b.retries = b.queue[1:], 0
	b.merged++
	if len(b.queue) == 0 {
		m.batch = nil
		return m.notify(fmt.Sprintf("Merged %d PRs", b.merged))
	}
	return m.batchMergeNext()
}
