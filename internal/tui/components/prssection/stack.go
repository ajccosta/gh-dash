package prssection

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

// Stack order (S, or a click on Base) shows how the PRs build on each other:
// a graph left of the list, read top-down in the order they can be merged.
// Each PR is right under the PR whose branch it is based on, one step to the
// right; PRs on a branch that is no open PR (main, mostly) start the stacks.
//
//	├─●  #13 check: print the install command
//	│ ├─●  #22 artifact.sh: --docker
//	│ │ └─●  #23 Build only the paper's allocators
//	│ └─●  #15 README: known crashes
//	└─●  #8 run_experiments.py: add the zipfian
//
// A PR stacked on one the section's search does not return still shows on
// it: the open PRs of its repo are fetched, and those it needs are added,
// greyed out.

const stackTitle = "Stack"

// stackColumn holds the graph; shown only in stack order.
func stackColumn() table.Column {
	return table.Column{Title: stackTitle, Width: utils.IntPtr(len(stackTitle) + 2), Hidden: utils.BoolPtr(true)}
}

// syncStackColumn shows the graph column, as wide as the graph, in stack order.
func (m *Model) syncStackColumn() {
	for i, c := range m.Table.Columns {
		if c.Title != stackTitle {
			continue
		}
		w := len(stackTitle)
		for _, pr := range m.Prs {
			w = max(w, len([]rune(pr.StackLanes[0]))+1)
		}
		m.Table.Columns[i].Width = utils.IntPtr(w + 2) // the cell padding
		m.Table.Columns[i].Hidden = utils.BoolPtr(m.sort != "base")
	}
}

// ToggleStacks puts the list in stack order (as a click on Base does), or
// back in GitHub's order.
func (m *Model) ToggleStacks() tea.Cmd {
	if m.sort != "base" {
		return m.SortBy("base")
	}
	m.sort, m.sortDesc = "", false
	data.SetSectionSort(m.sortId(), data.SectionSort{})
	m.ResetRows()
	m.syncStackColumn()
	return tea.Batch(m.FetchNextPageSectionRows()...)
}

// stackOrder arranges prs into stacks and draws each one's piece of the graph.
func stackOrder(prs []prrow.Data) {
	key := func(p *data.PullRequestData, branch string) [2]string {
		return [2]string{p.Repository.NameWithOwner, branch}
	}
	byHead := map[[2]string]int{}
	for i, pr := range prs {
		if p := pr.Primary; p != nil && !p.IsCrossRepository {
			byHead[key(p, p.HeadRefName)] = i
		}
	}
	parent := make([]int, len(prs))
	for i, pr := range prs {
		parent[i] = -1
		if p := pr.Primary; p != nil {
			if j, ok := byHead[key(p, p.BaseRefName)]; ok && j != i {
				parent[i] = j
			}
		}
	}
	// A cycle (a on b, b on a) starts its stack where it is first met.
	for i := range prs {
		for j, n := parent[i], 0; j >= 0 && n <= len(prs); j, n = parent[j], n+1 {
			if j == i {
				parent[i] = -1
				break
			}
		}
	}

	// The stacks on one branch hang from it, in the order of the list.
	children := make([][]int, len(prs))
	var groups [][2]string
	roots := map[[2]string][]int{}
	for i, pr := range prs {
		if parent[i] >= 0 {
			children[parent[i]] = append(children[parent[i]], i)
			continue
		}
		g := [2]string{}
		if p := pr.Primary; p != nil {
			g = key(p, p.BaseRefName)
		}
		if _, ok := roots[g]; !ok {
			groups = append(groups, g)
		}
		roots[g] = append(roots[g], i)
	}

	out := make([]prrow.Data, 0, len(prs))
	var walk func(i int, lanes string, last bool)
	walk = func(i int, lanes string, last bool) {
		pr := prs[i]
		branch, below := "├─", "│ "
		if last {
			branch, below = "└─", "  "
		}
		next := " "
		if len(children[i]) > 0 {
			next = "│"
		}
		pr.StackLanes = [2]string{lanes + branch, lanes + below + next}
		out = append(out, pr)
		for k, c := range children[i] {
			walk(c, lanes+below, k == len(children[i])-1)
		}
	}
	for _, g := range groups {
		for k, r := range roots[g] {
			walk(r, "", k == len(roots[g])-1)
		}
	}
	copy(prs, out)
}

// stackParentsMsg has the open PRs of the repos in the list.
type stackParentsMsg struct {
	prs []data.PullRequestData
}

// fetchStackParents fetches, in stack order, the open PRs of the repos in the
// list, for the PRs of the list to show on the ones they are stacked on.
func (m *Model) fetchStackParents() tea.Cmd {
	if m.sort != "base" {
		return nil
	}
	var repos []string
	for _, pr := range m.Prs {
		if r := pr.Primary.Repository.NameWithOwner; !pr.Extra && !slices.Contains(repos, r) {
			repos = append(repos, r)
		}
	}
	if len(repos) == 0 {
		return nil
	}
	taskId := fmt.Sprintf("fetching_stacks_%d_%d", m.Id, time.Now().UnixNano())
	start := m.Ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    "Fetching the PRs these are stacked on",
		FinishedText: "Fetched the PRs these are stacked on",
		State:        context.TaskStart,
	})
	sid, stype := m.Id, m.Type
	return tea.Batch(start, func() tea.Msg {
		var msg stackParentsMsg
		for _, r := range repos {
			res, err := data.FetchPullRequests("is:open repo:"+r, 100, nil)
			if err != nil {
				return constants.TaskFinishedMsg{SectionId: sid, SectionType: stype, TaskId: taskId, Err: err}
			}
			msg.prs = append(msg.prs, res.Prs...)
		}
		return constants.TaskFinishedMsg{SectionId: sid, SectionType: stype, TaskId: taskId, Msg: msg}
	})
}

// addStackParents adds, greyed out, the PRs the list is stacked on that it
// lacks, and those they are stacked on.
func (m *Model) addStackParents(msg stackParentsMsg) {
	if m.sort != "base" {
		return
	}
	m.Prs = slices.DeleteFunc(m.Prs, func(pr prrow.Data) bool { return pr.Extra })
	type ref struct {
		repo string
		n    int
	}
	have := map[ref]bool{}
	for _, pr := range m.Prs {
		have[ref{pr.Primary.Repository.NameWithOwner, pr.Primary.Number}] = true
	}
	byHead := map[[2]string]data.PullRequestData{}
	for _, p := range msg.prs {
		if !p.IsCrossRepository {
			byHead[[2]string{p.Repository.NameWithOwner, p.HeadRefName}] = p
		}
	}
	for i := 0; i < len(m.Prs); i++ { // m.Prs grows: the added ones' parents too
		p := m.Prs[i].Primary
		parent, ok := byHead[[2]string{p.Repository.NameWithOwner, p.BaseRefName}]
		if !ok || have[ref{parent.Repository.NameWithOwner, parent.Number}] {
			continue
		}
		have[ref{parent.Repository.NameWithOwner, parent.Number}] = true
		m.Prs = append(m.Prs, prrow.Data{Primary: &parent, Extra: true})
	}
	m.sortPrs()
}
