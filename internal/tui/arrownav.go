package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tabs"
)

// The "arrows" preview navigation (defaults.preview.navigation) treats the PRs
// view as four panes and moves between them with the arrow keys:
//
//	┌ views │ sections bar ────────────────┐   views: ←/→ switch view, ↓/enter → sections
//	│                                      │   sections: ←/→ switch section, ↑ → views,
//	│                                      │             ↓/enter/esc back down
//	├─ PR list ────────┬─ preview ─────────┤
//	│ ↑/↓ move         │ ←/→ tabs          │   list: ↑ on the first PR → sections,
//	│ → preview        │ ↑/↓ commit/scroll │         → → preview
//	│                  │ enter/d diff      │   preview: ← on the first tab → list,
//	└──────────────────┴───────────────────┘            ↑ at the top → button row
//	                                                    (or sections if it is empty)
//	button row (Approve, Merge): enter presses (as v / m do), ←/→ between them,
//	← on the first → list, ↑ → sections, ↓ → tabs
//
// Tab toggles between the list and the preview, Esc returns to the list. The
// focused bar shows its selected tab in reverse video. Every other key, and
// h/j/k/l, keep their usual meaning. The Issues and Notifications views get the
// views and sections bars and the list; their ←/→ and Tab keep their usual meaning.

type pane int

const (
	paneList pane = iota
	paneSections
	panePreview
	paneViews   // the view switcher left of the sections bar
	paneButtons // the button row at the top of the preview (prview.FocusedButton)
)

func (m *Model) arrowNavEnabled() bool {
	return m.ctx.Config.Defaults.Preview.Navigation == "arrows" &&
		m.ctx.View != config.RepoView
}

func (m *Model) setPane(p pane) {
	if p == paneButtons && m.prView.FocusedButton() == "" {
		// Not on a button yet: the first one, else the tabs.
		if ids := m.prView.ButtonIds(); len(ids) > 0 {
			m.prView.SetFocusedButton(ids[0])
		} else {
			p = panePreview
		}
	}
	if (p == panePreview || p == paneButtons) &&
		(!m.sidebar.IsOpen || m.ctx.View != config.PRsView) {
		p = paneList
	}
	if p == paneList || p == panePreview || p == paneButtons {
		m.paneBelowSections = p
	}
	m.pane = p
	m.tabs.SetFocused(p == paneSections)
	m.tabs.SetViewsFocused(p == paneViews)
	m.prView.SetFocused(p == panePreview || p == paneButtons)
	if p != paneButtons {
		m.prView.SetFocusedButton("")
	} else {
		m.prView.SetFocusedButton(m.prView.FocusedButton())
	}
	if p == paneButtons {
		m.sidebar.ScrollToTop()
	}
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
	if (m.pane == panePreview || m.pane == paneButtons) &&
		(!m.sidebar.IsOpen || m.ctx.View != config.PRsView) {
		m.setPane(paneList)
	}
	if m.pane == paneButtons && m.prView.ButtonAction(m.prView.FocusedButton()) == "" {
		// The focused button went away (another PR, or this one changed).
		m.prView.SetFocusedButton("")
		m.setPane(paneButtons)
	}
	prs := m.ctx.View == config.PRsView

	k := msg.String()
	switch k {
	case "tab":
		if !prs {
			return nil, false
		}
		if m.pane == panePreview || m.pane == paneButtons {
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
	case paneViews:
		switch k {
		case "left", "right":
			i := slices.Index(tabs.Views, m.ctx.View)
			if k == "left" {
				i--
			} else {
				i++
			}
			if i < 0 || i >= len(tabs.Views) {
				return nil, true
			}
			m.paneBelowSections = paneList
			cmd := m.switchToView(tabs.Views[i])
			m.setPane(paneViews)
			return cmd, true
		case "down", "enter":
			m.setPane(paneSections)
			return nil, true
		case "up":
			return nil, true
		}

	case paneSections:
		switch k {
		case "left", "right":
			return nil, false // the normal flow switches section
		case "down", "enter":
			m.setPane(m.paneBelowSections)
			return nil, true
		case "up":
			m.setPane(paneViews)
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
		case "left", "right":
			if !prs {
				return nil, false
			}
			if k == "right" && m.sidebar.IsOpen && m.getCurrRowData() != nil {
				m.setPane(panePreview)
			}
			return nil, true
		}

	case paneButtons:
		ids := m.prView.ButtonIds()
		i := slices.Index(ids, m.prView.FocusedButton())
		switch k {
		case "enter":
			cmd := m.pressButton(m.prView.FocusedButton())
			if m.prView.IsTextInputBoxFocused() { // Approve's prompt opened
				m.setPane(panePreview)
			}
			return cmd, true
		case "up":
			m.setPane(paneSections)
			return nil, true
		case "down":
			m.setPane(panePreview)
			return nil, true
		case "left":
			if i > 0 {
				m.prView.SetFocusedButton(ids[i-1])
			} else {
				m.setPane(paneList)
			}
			return nil, true
		case "right":
			if i+1 < len(ids) {
				m.prView.SetFocusedButton(ids[i+1])
			}
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
					m.setPane(m.paneAbovePreview())
					return nil, true
				}
				m.prView.MoveCommitCursor(delta)
				m.syncSidebar()
				m.ensureCommitCursorVisible()
			} else {
				if k == "up" && m.sidebar.YOffset() == 0 {
					m.setPane(m.paneAbovePreview())
					return nil, true
				}
				m.sidebar.ScrollLines(delta)
			}
			return nil, true

		case "enter", "d":
			if !m.prView.IsCommitsTab() {
				return nil, k == "enter"
			}
			oid, repo, prNumber := m.prView.SelectedCommit()
			if oid == "" {
				return nil, true
			}
			return common.DiffCommit(repo, oid, prNumber, m.ctx.Config.Pager.Diff,
				m.ctx.Config.GetFullScreenDiffPagerEnv(), m.ctx.Config.RepoPaths), true
		}
	}

	return nil, false
}

// paneAbovePreview is where ↑ at the top of the preview goes: the first
// button of the button row, else the sections bar.
func (m *Model) paneAbovePreview() pane {
	if len(m.prView.ButtonIds()) > 0 {
		return paneButtons
	}
	return paneSections
}

// pressButton does what the preview button with this zone id does, exactly
// as its key: Approve opens the approval prompt (v), the others ask "Are you
// sure you want to … this PR? (y/N)" before running gh (m merge, x close).
// Buttons call it on click and enter.
func (m *Model) pressButton(id string) tea.Cmd {
	action := m.prView.ButtonAction(id)
	if action == "" || m.prView.IsTextInputBoxFocused() {
		return nil
	}
	if action == "approve" {
		return m.openSidebarForPRInput(m.prView.SetIsApproving)
	}
	if m.ctx.View == config.PRsView {
		if m.getCurrRowData() == nil {
			return nil
		}
		return m.promptConfirmation(m.getCurrSection(), action)
	}
	if m.notificationView.GetSubjectPR() != nil {
		return m.promptConfirmationForNotificationPR(action)
	}
	return nil
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
