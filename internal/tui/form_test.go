package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

func TestKeybindingForm_ChoicesReachTheCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	zone.NewGlobal()
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	ran := 0
	var seen map[string]string
	run := func() tea.Cmd { ran++; seen = m.formValues; return nil }

	// Without a form the command runs at once.
	m.withForm(config.Keybinding{Key: "T"}, nil, run)
	require.Equal(t, 1, ran)
	require.Nil(t, seen)

	kb := config.Keybinding{Key: "T", Name: "review", Form: &config.KeybindingForm{
		Title: "Review in a worktree",
		Fields: []config.FormField{
			{Name: "Worktree", Options: []string{"persistent", "temporary"}},
			{Name: "Machine", Options: []string{"local"}, OptionsCommand: "printf 'tart\\npairloop-vm\\n'"},
		},
	}}
	pr := &prrow.Data{Primary: &data.PullRequestData{Number: 4, Title: "Build allocators"}}
	m.withForm(kb, pr, run)
	require.Equal(t, 1, ran, "the dialog asks first")
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "#4 Build allocators")
	require.Contains(t, view, "pairloop-vm", "optionsCommand adds options")

	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // temporary
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // tart
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, 2, ran)
	require.Equal(t, map[string]string{"Worktree": "temporary", "Machine": "tart"}, seen)
	require.Nil(t, m.formValues, "only while the command is made")

	// The next one starts from those choices.
	m.withForm(kb, pr, run)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, map[string]string{"Worktree": "temporary", "Machine": "tart"}, seen)
}

func TestKeybindingForm_ProgressBarThenRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	zone.NewGlobal()
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	kb := config.Keybinding{Key: "T", Name: "review", Form: &config.KeybindingForm{
		Fields:   []config.FormField{{Name: "Machine", Options: []string{"aware"}}},
		Progress: true,
	}}
	command := "echo 'progress 40 Pushing to aware'; echo noise; echo 'run echo shell'"
	m.withForm(kb, nil, func() tea.Cmd { return m.executeCustomCommand(command) })
	cmd := m.dialog.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	start, ok := cmd().(startProgressMsg)
	require.True(t, ok)
	require.Equal(t, command, start.command)
	next, _ := m.updateProgress(start)
	require.True(t, m.dialog.InProgress())

	line := next().(progressLineMsg)
	next, _ = m.updateProgress(line)
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "Pushing to aware")
	require.Contains(t, view, "40%")

	done := next().(progressDoneMsg)
	require.NoError(t, done.err)
	require.Equal(t, "echo shell", done.then)
	after, _ := m.updateProgress(done)
	require.False(t, m.dialog.IsOpen())
	require.NotNil(t, after, "the run line runs in the terminal")
}

func TestKeybindingForm_ProgressFailureShowsOutput(t *testing.T) {
	zone.NewGlobal()
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	next, _ := m.updateProgress(startProgressMsg{title: "Review", command: "echo 'progress 10 Copying'; echo 'cannot push' >&2; exit 3"})
	for {
		msg := next()
		if done, ok := msg.(progressDoneMsg); ok {
			m.updateProgress(done)
			break
		}
		next, _ = m.updateProgress(msg)
	}
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "cannot push")
	require.Contains(t, view, "Close")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.dialog.IsOpen())
}
