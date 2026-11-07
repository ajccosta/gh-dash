package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
)

// openMergeDialog asks how to merge the PR in a dialog over the dashboard,
// then merges it in the background: gh never takes over the terminal.
func (m *Model) openMergeDialog(sid tasks.SectionIdentifier, pr data.RowData) tea.Cmd {
	if pr == nil {
		return nil
	}
	text := []string{pr.GetTitle()}
	if row, ok := pr.(*prrow.Data); ok && row.Primary != nil && row.Primary.BaseRefName != "" {
		text = append(text, fmt.Sprintf("%s → %s", row.Primary.HeadRefName, row.Primary.BaseRefName))
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
			return tasks.MergePR(m.ctx, sid, pr, v["method"], v["delete"] == "yes")
		},
	})
	return nil
}
