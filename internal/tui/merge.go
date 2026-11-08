package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
)

// openMergeDialog asks how to merge the PR in a dialog over the dashboard,
// then merges it in the background: gh never takes over the terminal. Its
// branch is deleted as B does, moving the PRs based on it first. A PR GitHub
// can't merge but git can (prview.BaseMerge) gets its base merged in with git
// first.
func (m *Model) openMergeDialog(sid tasks.SectionIdentifier, pr data.RowData) tea.Cmd {
	if pr == nil {
		return nil
	}
	text := []string{pr.GetTitle()}
	if row, ok := pr.(*prrow.Data); ok && row.Primary != nil && row.Primary.BaseRefName != "" {
		text = append(text, fmt.Sprintf("%s → %s", row.Primary.HeadRefName, row.Primary.BaseRefName))
	}
	bm, viaGit := m.prView.BaseMerge()
	viaGit = viaGit && bm.Number == pr.GetNumber() && bm.Repo == pr.GetRepoNameWithOwner()
	if viaGit {
		text = append(text, "",
			"GitHub can't merge it, but git can: GitHub merges from one",
			"common ancestor, git from all of them. OK merges "+bm.Base,
			"into "+bm.Head+" with git first and pushes that; GitHub can",
			"merge the PR then.")
	}
	title := fmt.Sprintf("Merge #%d?", pr.GetNumber())
	if repo := pr.GetRepoNameWithOwner(); repo != "" && repo != "?" {
		title = fmt.Sprintf("Merge #%d in %s?", pr.GetNumber(), repo)
	}
	m.dialog.Open(dialog.Spec{
		Title: title,
		Text:  text,
		Fields: []dialog.Field{
			{Name: "method", Title: "Method", Options: []string{"merge", "squash", "rebase"}},
			{Name: "delete", Title: "Delete branch", Options: []string{"no", "yes"}},
		},
		Buttons: []string{"Merge", "Cancel"},
		Submit: func(v map[string]string, _ string) tea.Cmd {
			merge := func() tea.Cmd {
				if v["delete"] == "yes" {
					return m.planDeleteBranch(sid, pr, v["method"])
				}
				return tasks.MergePR(m.ctx, sid, pr, v["method"], nil)
			}
			if !viaGit {
				return merge()
			}
			return tasks.Then(m.ctx, sid, pr, "merge_base_in",
				fmt.Sprintf("Merging %s into %s with git", bm.Base, bm.Head),
				fmt.Sprintf("Merged %s into %s; GitHub can merge #%d", bm.Base, bm.Head, bm.Number),
				bm.Push, merge)
		},
	})
	return nil
}

// refreshSectionMsg refetches a section whose PRs a merge or a deleted branch
// changed: the merged PR leaves an open-PR search, the moved ones restack.
type refreshSectionMsg struct{ sid tasks.SectionIdentifier }

// refreshAfter refetches the section a little later, once GitHub's search
// has caught up with the change.
func refreshAfter(sid tasks.SectionIdentifier) tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return refreshSectionMsg{sid} })
}
