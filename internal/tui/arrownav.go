package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
)

// Keys of the "arrows" preview navigation (defaults.preview.navigation):
//
//	←/→        cycle the preview tabs (h/l still switch sections)
//	tab        focus the preview; tab or esc gives focus back to the list
//	↑/↓        with the preview focused: pick a commit on the Commits tab,
//	           scroll by one line on the other tabs
//	enter / d  with the preview focused on the Commits tab: that commit's diff
//
// Any other key keeps its usual meaning, focused or not.

func (m *Model) arrowNavEnabled() bool {
	return m.ctx.Config.Defaults.Preview.Navigation == "arrows" &&
		m.ctx.View == config.PRsView && m.sidebar.IsOpen
}

// handleArrowNav returns handled=false for keys it leaves to the normal flow.
func (m *Model) handleArrowNav(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !m.arrowNavEnabled() || m.mode == ModeSection || m.isUserDefinedKeybinding(msg) {
		m.prView.SetFocused(false)
		return nil, false
	}

	switch msg.String() {
	case "left", "right":
		delta := 1
		if msg.String() == "left" {
			delta = -1
		}
		m.prView.CycleTab(delta)
		m.syncSidebar()
		m.ensureCommitCursorVisible()
		return nil, true

	case "tab":
		focus := !m.prView.IsFocused() && m.getCurrRowData() != nil
		m.prView.SetFocused(focus)
		m.syncSidebar()
		m.ensureCommitCursorVisible()
		return nil, true
	}

	if !m.prView.IsFocused() {
		return nil, false
	}

	switch msg.String() {
	case "esc":
		m.prView.SetFocused(false)
		m.syncSidebar()
		return nil, true

	case "up", "down":
		delta := 1
		if msg.String() == "up" {
			delta = -1
		}
		if m.prView.IsCommitsTab() {
			m.prView.MoveCommitCursor(delta)
			m.syncSidebar()
			m.ensureCommitCursorVisible()
		} else {
			m.sidebar.ScrollLines(delta)
		}
		return nil, true

	case "enter", "d":
		if !m.prView.IsCommitsTab() {
			return nil, msg.String() == "enter"
		}
		oid, repo := m.prView.SelectedCommit()
		if oid == "" {
			return nil, true
		}
		return common.DiffCommit(repo, oid, m.ctx.Config.Pager.Diff,
			m.ctx.Config.GetFullScreenDiffPagerEnv()), true
	}

	return nil, false
}

// ensureCommitCursorVisible scrolls the preview to the selected commit.
func (m *Model) ensureCommitCursorVisible() {
	if !m.prView.IsFocused() || !m.prView.IsCommitsTab() {
		return
	}
	for i, line := range strings.Split(m.prView.View(), "\n") {
		if strings.Contains(line, prview.CommitCursorMarker) {
			m.sidebar.EnsureLineVisible(i)
			return
		}
	}
}
