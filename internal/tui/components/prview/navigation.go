package prview

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// State for the "arrows" preview navigation (defaults.preview.navigation):
// the pane can take focus, and while it has it a tab that lists things to
// open (the Commits tab, the Conflicts tab) keeps a cursor on one of them.

// CursorMarker starts the line of the selected entry. The UI looks for it in
// the rendered pane to keep the selection scrolled into view.
const CursorMarker = "▶"

// A picker is a section whose entries open on enter, d or a click: on its own
// tab ↑/↓ move a cursor over them, and wherever it shows they are clickable.
// The section is named as its tab. A new one is one entry in pickers.
type picker struct {
	tab   string // the tab, and the section, it is
	zone  string // zone id prefix of its entries
	count func(*Model) int
	open  func(*Model, int) tea.Cmd
}

var (
	commitPicker   = &picker{"Commits", "pr-commit-", (*Model).numCommitEntries, (*Model).openCommit}
	conflictPicker = &picker{"Conflicts", "pr-conflict-", (*Model).numConflictEntries, (*Model).openConflict}
	pickers        = []*picker{commitPicker, conflictPicker}
)

func (m *Model) IsFocused() bool {
	return m.focused
}

func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	m.carousel.SetHighlighted(focused && m.focusedButton == "")
}

// MoveTab moves to the next (delta > 0) or previous tab. It reports false,
// without moving, when there is no tab on that side.
func (m *Model) MoveTab(delta int) bool {
	next := m.carousel.Cursor() + delta
	if next < 0 || next >= len(tabs) {
		return false
	}
	m.carousel.SetCursor(next)
	return true
}

// tabPicker is the picker of the shown tab, or nil.
func (m *Model) tabPicker() *picker {
	for _, p := range pickers {
		if p.tab == m.carousel.SelectedItem() {
			return p
		}
	}
	return nil
}

// HasCursor tells whether the shown tab lists things to open.
func (m *Model) HasCursor() bool {
	return m.tabPicker() != nil
}

// Cursor is the selected entry of the shown tab; each tab starts at its
// first.
func (m *Model) Cursor() int {
	if m.cursorTab != m.carousel.SelectedItem() {
		return 0
	}
	return m.cursor
}

// MoveCursor moves the selected entry by delta, staying in range.
func (m *Model) MoveCursor(delta int) {
	p := m.tabPicker()
	if p == nil {
		return
	}
	m.cursor, m.cursorTab = 0, p.tab
	if n := p.count(m); n > 0 {
		m.cursor = min(max(m.Cursor()+delta, 0), n-1)
	}
}

// OpenSelected opens the selected entry of the shown tab.
func (m *Model) OpenSelected() tea.Cmd {
	p := m.tabPicker()
	if p == nil || m.pr == nil || m.pr.Data.Primary == nil {
		return nil
	}
	n := p.count(m)
	if n == 0 {
		return nil
	}
	return p.open(m, min(m.Cursor(), n-1))
}

// selected tells whether entry e of p has the cursor, shown only on p's tab
// (on the Overview ↑/↓ scroll and a click opens an entry).
func (m *Model) selected(p *picker, e int) bool {
	return m.focused && m.carousel.SelectedItem() == p.tab && e == m.Cursor()
}

// mark makes entry i of p clickable.
func mark(p *picker, i int, rendered string) string {
	if zone.DefaultManager == nil { // not set up (tests)
		return rendered
	}
	return zone.Mark(p.zone+strconv.Itoa(i), rendered)
}

// OpenAt opens the entry under the mouse, selecting it on its tab; ok false
// when there is none.
func (m *Model) OpenAt(msg tea.MouseMsg) (cmd tea.Cmd, ok bool) {
	if zone.DefaultManager == nil || m.pr == nil || m.pr.Data.Primary == nil {
		return nil, false
	}
	for _, p := range pickers {
		if !m.showsSection(p.tab) {
			continue
		}
		for i := range p.count(m) {
			if zone.Get(p.zone + strconv.Itoa(i)).InBounds(msg) {
				if m.carousel.SelectedItem() == p.tab {
					m.cursor, m.cursorTab = i, p.tab
				}
				return p.open(m, i), true
			}
		}
	}
	return nil, false
}

// TabAt returns the preview tab under the mouse, or -1.
func (m *Model) TabAt(msg tea.MouseMsg) int { return m.carousel.ItemAt(msg) }

// SelectTab shows tab i.
func (m *Model) SelectTab(i int) { m.carousel.SetCursor(i) }

// The Commits tab's entries: "All commits" (entry 0), then each commit
// (entry i+1 is commit i).
func (m *Model) numCommits() int {
	if m.pr == nil || !m.pr.Data.IsEnriched {
		return 0
	}
	return len(m.pr.Data.Enriched.AllCommits.Nodes)
}

func (m *Model) numCommitEntries() int {
	if n := m.numCommits(); n > 0 {
		return n + 1
	}
	return 0
}

// openCommit shows entry e's diff: a commit's alone, or for "All commits" the
// PR's in the diff pager with its Commits pane focused (ghdiff steps through
// them there).
func (m *Model) openCommit(e int) tea.Cmd {
	cfg := m.ctx.Config
	repo, prNumber := m.pr.Data.Primary.GetRepoNameWithOwner(), m.pr.Data.Primary.GetNumber()
	if e == 0 {
		return common.DiffPR(prNumber, repo, cfg.GetFullScreenDiffPagerEnv(), cfg.RepoPaths,
			"GHDIFF_PANE=commits")
	}
	return common.DiffCommit(repo, m.pr.Data.Enriched.AllCommits.Nodes[e-1].Commit.Oid, prNumber,
		cfg.Pager.Diff, cfg.GetFullScreenDiffPagerEnv(), cfg.RepoPaths)
}

// SelectedCommit is the commit with the cursor on the focused Commits tab:
// its oid and headline, or ok false ("All commits", or no cursor shown).
func (m *Model) SelectedCommit() (oid, headline string, ok bool) {
	e := m.Cursor()
	if e == 0 || e > m.numCommits() || !m.selected(commitPicker, e) {
		return "", "", false
	}
	c := m.pr.Data.Enriched.AllCommits.Nodes[e-1].Commit
	return c.Oid, c.MessageHeadline, true
}
