package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// stubGh puts a fake gh first on PATH: it prints stderr and exits with code.
// Nothing reaches GitHub.
func stubGh(t *testing.T, stderr string, code string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' '" + stderr + "' >&2\nexit " + code + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// runTask runs cmd (a tea.Batch of the start and the gh run) and returns the
// TaskFinishedMsg.
func runTask(t *testing.T, cmd tea.Cmd) constants.TaskFinishedMsg {
	t.Helper()
	var run func(tea.Cmd) *constants.TaskFinishedMsg
	run = func(c tea.Cmd) *constants.TaskFinishedMsg {
		if c == nil {
			return nil
		}
		switch msg := c().(type) {
		case constants.TaskFinishedMsg:
			return &msg
		case tea.BatchMsg:
			for _, sub := range msg {
				if r := run(sub); r != nil {
					return r
				}
			}
		}
		return nil
	}
	msg := run(cmd)
	require.NotNil(t, msg, "the task reports when gh finishes")
	return *msg
}

func TestApprovePR_FailureCarriesGhMessage(t *testing.T) {
	stubGh(t, "failed to create review: GraphQL: Can not approve your own pull request (addPullRequestReview)\n", "1")
	ctx := &context.ProgramContext{StartTask: func(context.Task) tea.Cmd { return nil }}

	msg := runTask(t, ApprovePR(ctx, SectionIdentifier{}, mockIssue{number: 4, repoName: "octocat/hello-world"}, ""))

	require.Error(t, msg.Err)
	require.Equal(t,
		"Approve #4 failed: failed to create review: GraphQL: Can not approve your own pull request (addPullRequestReview)",
		msg.Err.Error())
}

func TestApprovePR_Success(t *testing.T) {
	stubGh(t, "", "0")
	var started context.Task
	ctx := &context.ProgramContext{StartTask: func(task context.Task) tea.Cmd { started = task; return nil }}

	msg := runTask(t, ApprovePR(ctx, SectionIdentifier{}, mockIssue{number: 4, repoName: "o/r"}, "LGTM"))

	require.NoError(t, msg.Err)
	require.Equal(t, "Approving #4", started.StartText)
	require.Equal(t, "Approved #4", started.FinishedText)
}

func TestMergePR_RunsInBackgroundWithMethod(t *testing.T) {
	stubGh(t, "X Pull request octocat/x#4 is not mergeable: the merge commit cannot be cleanly created.\n", "1")
	ctx := &context.ProgramContext{StartTask: func(context.Task) tea.Cmd { return nil }}

	msg := runTask(t, MergePR(ctx, SectionIdentifier{}, mockIssue{number: 4, repoName: "octocat/x"}, "squash", nil))
	require.Equal(t,
		"Merge #4 failed: X Pull request octocat/x#4 is not mergeable: the merge commit cannot be cleanly created.",
		msg.Err.Error())
	require.False(t, *msg.Msg.(UpdatePRMsg).IsMerged)

	// gh gets the chosen method, so it asks nothing; the branch is not left to
	// gh, which would delete it with PRs still on it.
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"$*\" >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	del := &BranchDeletion{Branch: "feat/x", Base: "main", PRs: []int{5}}
	msg = runTask(t, MergePR(ctx, SectionIdentifier{}, mockIssue{number: 4, repoName: "octocat/x"}, "squash", del))
	require.Equal(t, "Merge #4 failed: pr merge 4 -R octocat/x --squash", msg.Err.Error())
	require.False(t, *msg.Msg.(UpdatePRMsg).IsMerged)
	require.Nil(t, msg.Msg.(UpdatePRMsg).BranchDeleted)

	stubGh(t, "", "0")
	msg = runTask(t, MergePR(ctx, SectionIdentifier{}, mockIssue{number: 4, repoName: "octocat/x"}, "merge", del))
	require.NoError(t, msg.Err)
	upd := msg.Msg.(UpdatePRMsg)
	require.True(t, *upd.IsMerged)
	require.True(t, *upd.BranchDeleted)
	require.Equal(t, map[int]string{5: "main"}, upd.Rebased)
}

func TestGhError_NoStderr(t *testing.T) {
	require.Nil(t, ghError(nil, "x", "p"))
	require.Equal(t, "Approve #1 failed: exit status 1",
		ghError(errors.New("exit status 1"), " \n", "Approve #1 failed").Error())
}
