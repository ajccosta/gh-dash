package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
)

// The "arrows" preview navigation (defaults.preview.navigation) treats the PRs
// view as three panes and moves between them with the arrow keys:
//
//	┌──────────── sections bar ────────────┐   ←/→ switch section, ↓/enter/esc back down
//	├─ PR list ────────┬─ preview ─────────┤
//	│ ↑/↓ move         │ ←/→ tabs          │   list: ↑ on the first PR → sections,
//	│ → preview        │ ↑/↓ commit/scroll │         → → preview
//	│                  │ enter/d diff      │   preview: ← on the first tab → list,
//	└──────────────────┴───────────────────┘            ↑ at the top → sections
//
// Tab toggles between the list and the preview, Esc returns to the list. The
// focused bar shows its selected tab in reverse video. Every other key, and
// h/j/k/l, keep their usual meaning.

type pane int

const (
	paneList pane = iota
	paneSections
	panePreview
)

func (m *Model) arrowNavEnabled() bool {
	return m.ctx.Config.Defaults.Preview.Navigation == "arrows" &&
		m.ctx.View == config.PRsView
}

func (m *Model) setPane(p pane) {
	if p == panePreview && !m.sidebar.IsOpen {
		p = paneList
	}
	if p != paneSections {
		m.paneBelowSections = p
	}
	m.pane = p
	m.tabs.SetFocused(p == paneSections)
	m.prView.SetFocused(p == panePreview)
	m.syncSidebar()
	m.ensureCommitCursorVisible()
}

// handleArrowNav returns handled=false for keys it leaves to the normal flow.
func (m *Model) handleArrowNav(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !m.arrowNavEnabled() || m.mode == ModeSection || m.isUserDefinedKeybinding(msg) {
		if m.pane != paneList {
			m.setPane(paneList)
		}
		return nil, false
	}
	if m.pane == panePreview && !m.sidebar.IsOpen {
		m.setPane(paneList)
	}

	k := msg.String()
	switch k {
	case "tab":
		if m.pane == panePreview {
			m.setPane(paneList)
		} else if m.getCurrRowData() != nil {
			m.setPane(panePreview)
		}
		return nil, true
	case "esc":
		if m.pane != paneList {
			m.setPane(paneList)
			return nil, true
		}
		return nil, false
	}

	switch m.pane {
	case paneSections:
		switch k {
		case "left", "right":
			return nil, false // the normal flow switches section
		case "down", "enter":
			m.setPane(m.paneBelowSections)
			return nil, true
		case "up":
			return nil, true
		}

	case paneList:
		switch k {
		case "up":
			if s := m.getCurrSection(); s == nil || s.CurrRow() <= 0 {
				m.setPane(paneSections)
				return nil, true
			}
			return nil, false
		case "right":
			if m.sidebar.IsOpen && m.getCurrRowData() != nil {
				m.setPane(panePreview)
			}
			return nil, true
		case "left":
			return nil, true
		}

	case panePreview:
		switch k {
		case "left", "right":
			delta := 1
			if k == "left" {
				delta = -1
			}
			if !m.prView.MoveTab(delta) {
				if k == "left" {
					m.setPane(paneList)
				}
				return nil, true
			}
			m.syncSidebar()
			m.ensureCommitCursorVisible()
			return nil, true

		case "up", "down":
			delta := 1
			if k == "up" {
				delta = -1
			}
			if m.prView.IsCommitsTab() {
				if k == "up" && m.prView.CommitCursor() == 0 {
					m.setPane(paneSections)
					return nil, true
				}
				m.prView.MoveCommitCursor(delta)
				m.syncSidebar()
				m.ensureCommitCursorVisible()
			} else {
				if k == "up" && m.sidebar.YOffset() == 0 {
					m.setPane(paneSections)
					return nil, true
				}
				m.sidebar.ScrollLines(delta)
			}
			return nil, true

		case "enter", "d":
			if !m.prView.IsCommitsTab() {
				return nil, k == "enter"
			}
			oid, repo := m.prView.SelectedCommit()
			if oid == "" {
				return nil, true
			}
			return common.DiffCommit(repo, oid, m.ctx.Config.Pager.Diff,
				m.ctx.Config.GetFullScreenDiffPagerEnv()), true
		}
	}

	return nil, false
}

// ensureCommitCursorVisible scrolls the preview to the selected commit.
func (m *Model) ensureCommitCursorVisible() {
	if m.pane != panePreview || !m.prView.IsCommitsTab() {
		return
	}
	for i, line := range strings.Split(m.prView.View(), "\n") {
		if strings.Contains(line, prview.CommitCursorMarker) {
			m.sidebar.EnsureLineVisible(i)
			return
		}
	}
}
