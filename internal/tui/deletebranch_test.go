package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// stubGh puts a gh on PATH that logs its arguments and answers as GitHub
// would for o/r: #2 and #3 based on feat/x, main the default branch. With
// fail set, `gh pr edit` that many PRs in fails.
func stubGh(t *testing.T, fail string) string {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	script := `#!/bin/sh
echo "$*" >> '` + log + `'
case "$1 $2" in
"pr list") echo '[{"number":2,"title":"Second"},{"number":3,"title":"Third"}]' ;;
"repo view") echo main ;;
"pr edit") [ "$3" = "` + fail + `" ] && { echo "cannot edit" >&2; exit 1; } ;;
esac
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func ghCalls(t *testing.T, log string) []string {
	b, err := os.ReadFile(log)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// finished runs a task's command and returns what its gh calls ended in.
func finished(t *testing.T, cmd tea.Cmd) constants.TaskFinishedMsg {
	msg := cmd()
	if f, ok := msg.(constants.TaskFinishedMsg); ok { // tea.Batch of one
		return f
	}
	for _, c := range msg.(tea.BatchMsg) {
		if c == nil {
			continue
		}
		if f, ok := c().(constants.TaskFinishedMsg); ok {
			return f
		}
	}
	t.Fatal("no TaskFinishedMsg")
	return constants.TaskFinishedMsg{}
}

func mergedPR(head string) *prrow.Data {
	pr := &data.PullRequestData{Number: 1, Title: "First", State: "MERGED",
		HeadRefName: head, BaseRefName: "main"}
	pr.HeadRef.Name = head
	pr.Repository.NameWithOwner = "o/r"
	return &prrow.Data{Primary: pr}
}

func TestDeleteBranch_MovesStackedPRsThenDeletes(t *testing.T) {
	zone.NewGlobal()
	log := stubGh(t, "")
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	sid := tasks.SectionIdentifier{Id: 0, Type: "pr"}

	plan := m.planDeleteBranch(sid, mergedPR("feat/x"), "")().(deleteBranchPlanMsg)
	require.NoError(t, plan.err)
	require.Nil(t, m.openDeleteBranchDialog(plan))
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "Delete branch feat/x of #1?")
	require.Contains(t, view, "#2 Second")
	require.Contains(t, view, "#3 Third")
	require.Contains(t, view, "onto main")

	cmd := m.dialog.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	done := finished(t, cmd)
	require.NoError(t, done.Err)
	require.Equal(t, []string{
		"pr list -R o/r --base feat/x --state open --json number,title --limit 200",
		"repo view o/r --json defaultBranchRef --jq .defaultBranchRef.name",
		"pr edit 2 -R o/r --base main",
		"pr edit 3 -R o/r --base main",
		"api -X DELETE repos/o/r/git/refs/heads/feat/x",
	}, ghCalls(t, log))
	upd := done.Msg.(tasks.UpdatePRMsg)
	require.True(t, *upd.BranchDeleted)
	require.Equal(t, map[int]string{2: "main", 3: "main"}, upd.Rebased)
}

func TestDeleteBranch_KeepsBranchWhenAPRCannotMove(t *testing.T) {
	log := stubGh(t, "3")
	m := newApproveTestModel(t, "other")
	sid := tasks.SectionIdentifier{Id: 0, Type: "pr"}
	done := finished(t, tasks.DeleteBranch(m.ctx, sid, mergedPR("feat/x"),
		tasks.BranchDeletion{Branch: "feat/x", Base: "main", PRs: []int{2, 3}}))
	require.ErrorContains(t, done.Err, "cannot edit")
	calls := ghCalls(t, log)
	require.Equal(t, "pr edit 3 -R o/r --base main", calls[len(calls)-1], "no delete after it")
	require.False(t, *done.Msg.(tasks.UpdatePRMsg).BranchDeleted)
}

func TestDeleteBranch_NeverTheDefaultBranch(t *testing.T) {
	log := stubGh(t, "")
	m := newApproveTestModel(t, "other")
	sid := tasks.SectionIdentifier{Id: 0, Type: "pr"}
	plan := m.planDeleteBranch(sid, mergedPR("main"), "")().(deleteBranchPlanMsg)
	err := m.openDeleteBranchDialog(plan)().(constants.ErrMsg).Err
	require.ErrorContains(t, err, "default branch")
	require.False(t, m.dialog.IsOpen())
	for _, c := range ghCalls(t, log) {
		require.NotContains(t, c, "DELETE")
	}

	open := mergedPR("feat/x")
	open.Primary.State = "OPEN"
	err = m.planDeleteBranch(sid, open, "")().(constants.ErrMsg).Err
	require.ErrorContains(t, err, "merged")
}

func TestMergeDeletingBranch_MovesStackedPRsFirst(t *testing.T) {
	log := stubGh(t, "")
	m := newApproveTestModel(t, "other")
	sid := tasks.SectionIdentifier{Id: 0, Type: "pr"}
	open := mergedPR("feat/x")
	open.Primary.State = "OPEN"

	plan := m.planDeleteBranch(sid, open, "squash")().(deleteBranchPlanMsg)
	cmd := m.openDeleteBranchDialog(plan)
	require.False(t, m.dialog.IsOpen(), "the merge dialog asked already")
	done := finished(t, cmd)
	require.NoError(t, done.Err)
	require.Equal(t, []string{
		"pr list -R o/r --base feat/x --state open --json number,title --limit 200",
		"repo view o/r --json defaultBranchRef --jq .defaultBranchRef.name",
		"pr merge 1 -R o/r --squash",
		"pr edit 2 -R o/r --base main",
		"pr edit 3 -R o/r --base main",
		"api -X DELETE repos/o/r/git/refs/heads/feat/x",
	}, ghCalls(t, log))
	upd := done.Msg.(tasks.UpdatePRMsg)
	require.True(t, *upd.IsMerged)
	require.True(t, *upd.BranchDeleted)
}
