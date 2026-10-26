package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// Mouse support (defaults.mouse): a left click on a view, a section tab, the
// "+" button, a list row or a preview tab does what the keys for it do. Every
// clickable thing is a bubblezone mark made where it is drawn; this maps the
// click back to it. Approve, Merge and the donate link are handled in Update.

// handleMouseClick reports whether the click landed on something it handles.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (bool, tea.Cmd) {
	if s := m.getCurrSection(); s != nil && s.IsSearchFocused() && !s.SearchAt(msg) {
		s.SetIsSearching(false) // clicking away leaves the search box, as Esc does
	}
	if v, ok := m.tabs.ViewAt(msg); ok {
		if v == m.ctx.View {
			return true, nil
		}
		m.paneBelowSections = paneList
		cmd := m.switchToView(v)
		m.setPane(paneList)
		return true, cmd
	}
	if m.tabs.NewSectionAt(msg) {
		m.addNewSection()
		return true, nil
	}
	if i := m.tabs.SectionAt(msg); i >= 0 {
		if s := m.getSectionAt(i); s != nil && i != m.currSectionId {
			m.setCurrSectionId(s.GetId())
			return true, m.onViewedRowChanged()
		}
		return true, nil
	}
	if m.ctx.View == config.PRsView && m.sidebar.IsOpen {
		if i := m.prView.TabAt(msg); i >= 0 {
			m.prView.SelectTab(i)
			m.syncSidebar()
			return true, nil
		}
	}
	if m.ctx.View == config.PRsView && m.sidebar.IsOpen {
		if i := m.prView.CommitAt(msg); i >= 0 {
			m.prView.SelectCommit(i)
			oid, repo, prNumber := m.prView.SelectedCommit()
			if oid == "" {
				return true, nil
			}
			return true, common.DiffCommit(repo, oid, prNumber, m.ctx.Config.Pager.Diff,
				m.ctx.Config.GetFullScreenDiffPagerEnv(), m.ctx.Config.RepoPaths)
		}
	}
	if s := m.getCurrSection(); s != nil && s.SearchAt(msg) {
		if s.IsSearchFocused() {
			return true, nil
		}
		return true, s.SetIsSearching(true)
	}
	if s := m.getCurrSection(); s != nil {
		if i := s.RowAt(msg); i >= 0 {
			if i == s.CurrRow() {
				return true, nil
			}
			s.SelectRow(i)
			return true, m.onViewedRowChanged()
		}
	}
	return false, nil
}

// handleMouseWheel scrolls the preview when the pointer is over it, and moves
// the list selection otherwise.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Cmd, bool) {
	var delta int
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = 1
	default:
		return nil, false
	}
	if m.sidebar.IsOpen && m.overSidebar(msg.Mouse()) {
		m.sidebar.ScrollLines(3 * delta)
		return nil, true
	}
	s := m.getCurrSection()
	if s == nil {
		return nil, false
	}
	before := s.CurrRow()
	if delta > 0 {
		s.NextRow()
	} else {
		s.PrevRow()
	}
	if s.CurrRow() == before {
		return nil, true
	}
	cmds := []tea.Cmd{m.onViewedRowChanged()}
	if delta > 0 && s.CurrRow() == s.NumRows()-1 && m.ctx.View != config.RepoView {
		cmds = append(cmds, s.FetchNextPageSectionRows()...)
	}
	return tea.Batch(cmds...), true
}

func (m *Model) overSidebar(p tea.Mouse) bool {
	if m.ctx.PreviewPosition == "bottom" {
		return p.Y >= common.HeaderHeight+m.ctx.MainContentHeight
	}
	return p.X >= m.ctx.MainContentWidth
}
