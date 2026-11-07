package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

func TestKeybindingForm_ChoicesReachTheCommand(t *testing.T) {
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
}
