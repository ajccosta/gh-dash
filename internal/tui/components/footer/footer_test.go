package footer

import (
	"strings"
	"testing"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

func TestHintShowsFooterKeybindings(t *testing.T) {
	cfg := config.Config{}
	cfg.Keybindings.Prs = []config.Keybinding{
		{Key: "T", Name: "review in a worktree", Command: "true", Footer: true},
		{Key: "D", Name: "remove its worktree", Command: "true"},
	}
	m := Model{ctx: &context.ProgramContext{Config: &cfg, View: config.PRsView}}
	hint := m.viewHint()
	if !strings.Contains(hint, "T review in a worktree") {
		t.Errorf("hint %q lacks the footer keybinding", hint)
	}
	if strings.Contains(hint, "remove its worktree") {
		t.Errorf("hint %q shows a keybinding without footer: true", hint)
	}
	m.ctx.View = config.IssuesView
	if hint := m.viewHint(); strings.Contains(hint, "T review") {
		t.Errorf("issues hint %q shows a PR keybinding", hint)
	}
}
