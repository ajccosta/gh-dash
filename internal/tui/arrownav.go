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
	paneApprove // the approve button at the top of the preview
	paneMerge   // the merge button, right of the approve button
)

func isButtonPane(p pane) bool { return p == paneApprove || p == paneMerge }

func (m *Model) arrowNavEnabled() bool {
	return m.ctx.Config.Defaults.Preview.Navigation == "arrows" &&
		m.ctx.View != config.RepoView
}

func (m *Model) setPane(p pane) {
	if p == paneApprove && !m.prView.CanApprove() {
		p = panePreview
	}
	if p == paneMerge && !m.prView.CanMerge() {
		p = panePreview
	}
	if (p == panePreview || isButtonPane(p)) &&
		(!m.sidebar.IsOpen || m.ctx.View != config.PRsView) {
		p = paneList
	}
	if p == paneList || p == panePreview || isButtonPane(p) {
		m.paneBelowSections = p
	}
	m.pane = p
	m.tabs.SetFocused(p == paneSections)
	m.tabs.SetViewsFocused(p == paneViews)
	m.prView.SetFocused(p == panePreview || isButtonPane(p))
	m.prView.SetApproveFocused(p == paneApprove)
	m.prView.SetMergeFocused(p == paneMerge)
	if isButtonPane(p) {
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
	if (m.pane == panePreview || isButtonPane(m.pane)) &&
		(!m.sidebar.IsOpen || m.ctx.View != config.PRsView) {
		m.setPane(paneList)
	}
	if (m.pane == paneApprove && !m.prView.CanApprove()) ||
		(m.pane == paneMerge && !m.prView.CanMerge()) {
		m.setPane(panePreview)
	}
	prs := m.ctx.View == config.PRsView

	k := msg.String()
	switch k {
	case "tab":
		if !prs {
			return nil, false
		}
		if m.pane == panePreview || isButtonPane(m.pane) {
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

	case paneApprove:
		switch k {
		case "enter":
			m.setPane(panePreview)
			return m.approveCurrentPR(), true
		case "up":
			m.setPane(paneSections)
			return nil, true
		case "down":
			m.setPane(panePreview)
			return nil, true
		case "left":
			m.setPane(paneList)
			return nil, true
		case "right":
			if m.prView.CanMerge() {
				m.setPane(paneMerge)
			}
			return nil, true
		}

	case paneMerge:
		switch k {
		case "enter":
			return m.mergeCurrentPR(), true
		case "up":
			m.setPane(paneSections)
			return nil, true
		case "down":
			m.setPane(panePreview)
			return nil, true
		case "left":
			if m.prView.CanApprove() {
				m.setPane(paneApprove)
			} else {
				m.setPane(paneList)
			}
			return nil, true
		case "right":
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
// button of the button row (Approve, then Merge), else the sections bar.
func (m *Model) paneAbovePreview() pane {
	if m.prView.CanApprove() {
		return paneApprove
	}
	if m.prView.CanMerge() {
		return paneMerge
	}
	return paneSections
}

// mergeCurrentPR asks to merge the previewed PR, exactly as the Merge key (m)
// does: the "Are you sure you want to merge this PR? (y/N)" prompt, then
// `gh pr merge`. The merge button calls it on click and enter.
func (m *Model) mergeCurrentPR() tea.Cmd {
	if !m.prView.CanMerge() || m.prView.IsTextInputBoxFocused() {
		return nil
	}
	if m.ctx.View == config.PRsView {
		if m.getCurrRowData() == nil {
			return nil
		}
		return m.promptConfirmation(m.getCurrSection(), "merge")
	}
	if m.notificationView.GetSubjectPR() != nil {
		return m.promptConfirmationForNotificationPR("merge")
	}
	return nil
}

// approveCurrentPR opens the approval prompt for the previewed PR, exactly as
// the Approve key (v) does. The approve button calls it on click and enter.
func (m *Model) approveCurrentPR() tea.Cmd {
	if !m.prView.CanApprove() || m.prView.IsTextInputBoxFocused() {
		return nil
	}
	return m.openSidebarForPRInput(m.prView.SetIsApproving)
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
