package tui

import (
	"reflect"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// Mouse support (defaults.mouse): a left click on a view, a section tab, the
// "+" button, a list row or a preview tab does what the keys for it do. Every
// clickable thing is a bubblezone mark made where it is drawn; this maps the
// click back to it. Approve, Merge and the donate link are handled in Update.

// handleMouseClick reports whether the click landed on something it handles.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (bool, tea.Cmd) {
	if url := m.linkAt(msg); url != "" && msg.Button == tea.MouseLeft {
		return true, openURL(url)
	}
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
	if pr := m.previewPR(); pr != nil {
		if i := m.prView.TabAt(msg); i >= 0 {
			m.prView.SelectTab(i)
			m.syncSidebar()
			return true, nil
		}
		if loc, ok := m.prView.CodeLocAt(msg); ok {
			return true, common.DiffPR(pr.GetNumber(), pr.GetRepoNameWithOwner(),
				m.ctx.Config.GetFullScreenDiffPagerEnv(), m.ctx.Config.RepoPaths, loc.GotoEnv())
		}
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
		if k := s.HeaderAt(msg); k != "" {
			return true, tea.Batch(s.SortBy(k), m.onViewedRowChanged())
		}
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

// Wheel events come in bursts (a trackpad flick sends dozens), and moving
// the selection re-renders the preview and fetches the PR's details. So a
// burst is added up and applied once per frame: handleMouseWheel counts,
// applyWheel moves, and View shows the last frame until then.

const wheelFrame = time.Second / 60

// wheelState is the scrolling counted since the last frame.
type wheelState struct {
	list, preview int // rows to move the selection, lines to scroll the preview
	pending       bool
}

type wheelFlushMsg struct{}

// handleMouseWheel counts a wheel step: over the preview it scrolls it,
// elsewhere it moves the list selection.
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
		m.wheel.preview += 3 * delta
	} else if m.getCurrSection() != nil {
		m.wheel.list += delta
	} else {
		return nil, false
	}
	if m.wheel.pending {
		return nil, true
	}
	m.wheel.pending = true
	return tea.Tick(wheelFrame, func(time.Time) tea.Msg { return wheelFlushMsg{} }), true
}

// applyWheel does the scrolling counted since the last frame.
func (m *Model) applyWheel() tea.Cmd {
	w := m.wheel
	m.wheel = wheelState{}
	if w.preview != 0 {
		m.sidebar.ScrollLines(w.preview)
	}
	s := m.getCurrSection()
	if w.list == 0 || s == nil || s.NumRows() == 0 {
		return nil
	}
	before := s.CurrRow()
	s.SelectRow(min(max(before+w.list, 0), s.NumRows()-1))
	if s.CurrRow() == before {
		return nil
	}
	cmds := []tea.Cmd{m.onViewedRowChanged()}
	if w.list > 0 && s.CurrRow() == s.NumRows()-1 && m.ctx.View != config.RepoView {
		cmds = append(cmds, s.FetchNextPageSectionRows()...)
	}
	return tea.Batch(cmds...)
}

func (m *Model) overSidebar(p tea.Mouse) bool {
	if m.ctx.PreviewPosition == "bottom" {
		return p.Y >= common.HeaderHeight+m.ctx.MainContentHeight
	}
	return p.X >= m.ctx.MainContentWidth
}

// previewPR is the PR the preview shows (in the PRs view, or as a
// notification's subject), or nil.
func (m *Model) previewPR() data.RowData {
	if !m.sidebar.IsOpen {
		return nil
	}
	if m.ctx.View == config.NotificationsView {
		if p := m.notificationView.GetSubjectPR(); p != nil {
			return p
		}
		return nil
	}
	if m.ctx.View == config.PRsView {
		if pr := m.getCurrRowData(); pr != nil && !reflect.ValueOf(pr).IsNil() {
			return pr
		}
	}
	return nil
}
