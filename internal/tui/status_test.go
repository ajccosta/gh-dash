package tui

import (
	"errors"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

func newStatusTestModel(t *testing.T, author string) Model {
	m := newApproveTestModel(t, author)
	m.tasks = map[string]context.Task{}
	m.taskSpinner = spinner.New()
	m.ctx.ScreenWidth = 160
	m.ctx.StartTask = func(task context.Task) tea.Cmd {
		task.StartTime = time.Now()
		m.tasks[task.Id] = task
		return nil
	}
	return m
}

func footerText(m Model) string { return ansi.Strip(m.footer.View()) }

func TestTaskStatus_ErrorShowsGhMessage(t *testing.T) {
	m := newStatusTestModel(t, "other")
	m.ctx.StartTask(context.Task{Id: "pr_approve_4", StartText: "Approving #4",
		FinishedText: "Approved #4", State: context.TaskStart})
	// A background fetch still running must not hide the failure.
	m.ctx.StartTask(context.Task{Id: "fetch", StartText: "Fetching PRs", State: context.TaskStart})

	m.Update(constants.TaskFinishedMsg{TaskId: "pr_approve_4",
		Err: errors.New("Approve #4 failed: failed to create review: GraphQL: Can not approve your own pull request (addPullRequestReview)")})

	f := footerText(m)
	require.Contains(t, f, "✗ Approve #4 failed: failed to create review: GraphQL: Can not approve")
	require.NotContains(t, f, "\n", "the status stays on the footer line")
	require.Equal(t, 160, ansi.StringWidth(f))

	// The early clear of another task does not wipe it.
	m.Update(constants.ClearTaskMsg{TaskId: "pr_approve_4"})
	require.Contains(t, footerText(m), "Approve #4 failed")
}

func TestTaskStatus_SuccessShowsConfirmation(t *testing.T) {
	m := newStatusTestModel(t, "other")
	m.ctx.StartTask(context.Task{Id: "pr_approve_4", StartText: "Approving #4",
		FinishedText: "Approved #4", State: context.TaskStart})
	m.Update(constants.TaskFinishedMsg{TaskId: "pr_approve_4"})
	require.Contains(t, footerText(m), "✓ Approved #4")
}

func TestApproveKey_OwnPRShowsError(t *testing.T) {
	m := newStatusTestModel(t, "me")
	m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	require.False(t, m.prView.GetIsApproving(), "v does not open the prompt on your own PR")
	require.Contains(t, footerText(m), "✗ Can't approve your own PR #1")
}

func TestFooterHints_ApproveAndMerge(t *testing.T) {
	m := newStatusTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.footer.SetPRActions(true, true)
	require.Contains(t, footerText(m), "v approve · m merge")
	m.footer.SetPRActions(false, true) // own PR
	require.NotContains(t, footerText(m), "v approve")
	require.Contains(t, footerText(m), "m merge")
}
