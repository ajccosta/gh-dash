package tui

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// How long a finished task stays in the footer. Errors stay longer, so that
// gh's message (e.g. "Can not approve your own pull request") can be read.
const (
	taskSuccessShownFor = 3 * time.Second
	taskErrorShownFor   = 12 * time.Second
)

func taskShownFor(task context.Task) time.Duration {
	if task.State == context.TaskError {
		return taskErrorShownFor
	}
	return taskSuccessShownFor
}

func clearTaskAfter(id string, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return constants.ClearTaskMsg{TaskId: id}
	})
}

// refreshTaskStatus redraws the footer's task status now, rather than on the
// next spinner tick, and clears it when no task is left.
func (m *Model) refreshTaskStatus() {
	if len(m.tasks) == 0 {
		m.footer.SetRightSection("")
		return
	}
	m.footer.SetRightSection(m.renderRunningTask())
}

// showStatusError shows text as a failed task in the footer.
func (m *Model) showStatusError(id, text string) tea.Cmd {
	now := time.Now()
	m.tasks[id] = context.Task{
		Id:           id,
		State:        context.TaskError,
		Error:        errors.New(text),
		StartTime:    now,
		FinishedTime: &now,
	}
	m.refreshTaskStatus()
	return clearTaskAfter(id, taskErrorShownFor)
}

// approvePRKey handles the Approve key (v): it opens the approval prompt,
// except on your own PR, which GitHub refuses to approve.
func (m *Model) approvePRKey() tea.Cmd {
	if m.prView.IsOwnPR() {
		return m.showStatusError("approve_own",
			fmt.Sprintf("Can't approve your own PR #%d", m.prView.PRNumber()))
	}
	return m.openSidebarForPRInput(m.prView.SetIsApproving)
}
