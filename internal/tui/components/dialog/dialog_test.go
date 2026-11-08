package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newDialog(submitted *map[string]string) Model {
	zone.NewGlobal()
	ctx := &context.ProgramContext{ScreenWidth: 100, ScreenHeight: 30, Theme: *theme.DefaultTheme}
	m := New(ctx)
	m.Open(Spec{
		Title: "Pick",
		Fields: []Field{
			{Name: "a", Title: "A", Options: []string{"x", "y"}},
			{Name: "b", Title: "B", Options: []string{"1", "2", "3"}},
		},
		Buttons: []string{"Go", "Cancel"},
		Submit: func(v map[string]string, _ string) tea.Cmd {
			*submitted = v
			return func() tea.Msg { return nil }
		},
	})
	return m
}

func TestDialog_KeysChooseAndConfirm(t *testing.T) {
	var got map[string]string
	m := newDialog(&got)
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // a: y
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})  // to b
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})  // b wraps to 3
	require.NotNil(t, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Equal(t, map[string]string{"a": "y", "b": "3"}, got)
	require.False(t, m.IsOpen())
}

func TestDialog_CancelRunsNothing(t *testing.T) {
	var got map[string]string
	m := newDialog(&got)
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // the buttons row
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Nil(t, got)
	require.False(t, m.IsOpen())

	m = newDialog(&got)
	require.Nil(t, m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}))
	require.False(t, m.IsOpen())
}

func TestDialog_LongOptionRowScrolls(t *testing.T) {
	f := Field{Options: strings.Fields("local aware cluster tart grid grenoble lille louvain luxembourg lyon nancy nantes")}
	lo, hi := visibleOptions(f, 30)
	require.Equal(t, 0, lo)
	require.Less(t, hi, len(f.Options)-1)

	f.Sel = len(f.Options) - 1
	lo, hi = visibleOptions(f, 30)
	require.Equal(t, len(f.Options)-1, hi, "the chosen option is always shown")
	require.Greater(t, lo, 0)

	var got map[string]string
	m := newDialog(&got)
	m.spec.Fields[0].Options = f.Options
	view := ansi.Strip(m.View())
	require.Contains(t, view, "›", "hidden options are marked")
	for _, l := range strings.Split(view, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(l), 100)
	}
}

func TestDialog_RemembersChoices(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	zone.NewGlobal()
	m := New(&context.ProgramContext{ScreenWidth: 100, ScreenHeight: 30, Theme: *theme.DefaultTheme})
	open := func() {
		m.Open(Spec{
			Title:    "Pick",
			Fields:   []Field{{Name: "a", Title: "A", Options: []string{"x", "y"}}},
			Buttons:  []string{"Go", "Cancel"},
			Submit:   func(map[string]string, string) tea.Cmd { return nil },
			Remember: "dialog test",
		})
	}
	open()
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // y
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	open()
	require.Equal(t, "y", m.Values()["a"], "starts at the last choice")

	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // x, then cancel
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	open()
	require.Equal(t, "y", m.Values()["a"], "a cancelled dialog is not remembered")
}
