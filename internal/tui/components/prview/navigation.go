package prview

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// State for the "arrows" preview navigation (defaults.preview.navigation):
// the pane can take focus, and while it has it the Commits tab keeps a cursor
// on one commit so its diff can be opened on its own.

// CommitCursorMarker starts the line of the selected commit. The UI looks for
// it in the rendered pane to keep the selection scrolled into view.
const CommitCursorMarker = "▶"

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

func (m *Model) CommitCursor() int {
	return m.commitCursor
}

func (m *Model) IsCommitsTab() bool {
	return m.carousel.SelectedItem() == "Commits"
}

func (m *Model) numCommits() int {
	if m.pr == nil || !m.pr.Data.IsEnriched {
		return 0
	}
	return len(m.pr.Data.Enriched.AllCommits.Nodes)
}

// The Commits tab's entries: "All commits" (entry 0), then each commit
// (entry i+1 is commit i).
func (m *Model) numCommitEntries() int {
	if n := m.numCommits(); n > 0 {
		return n + 1
	}
	return 0
}

// MoveCommitCursor moves the selected entry by delta, staying in range.
func (m *Model) MoveCommitCursor(delta int) {
	n := m.numCommitEntries()
	if n == 0 {
		m.commitCursor = 0
		return
	}
	m.commitCursor = min(max(m.commitCursor+delta, 0), n-1)
}

// OpenSelectedCommit shows the selected entry's diff: a commit's alone, or for
// "All commits" the PR's in the diff pager with its Commits pane focused
// (ghdiff steps through them there).
func (m *Model) OpenSelectedCommit() tea.Cmd {
	n := m.numCommitEntries()
	if n == 0 || m.pr.Data.Primary == nil {
		return nil
	}
	cfg := m.ctx.Config
	repo, prNumber := m.pr.Data.Primary.GetRepoNameWithOwner(), m.pr.Data.Primary.GetNumber()
	e := min(m.commitCursor, n-1)
	if e == 0 {
		return common.DiffPR(prNumber, repo, cfg.GetFullScreenDiffPagerEnv(), cfg.RepoPaths,
			"GHDIFF_PANE=commits")
	}
	return common.DiffCommit(repo, m.pr.Data.Enriched.AllCommits.Nodes[e-1].Commit.Oid, prNumber,
		cfg.Pager.Diff, cfg.GetFullScreenDiffPagerEnv(), cfg.RepoPaths)
}

// commitSelected tells whether entry e has the cursor, shown only on the
// Commits tab (on the Overview ↑/↓ scroll and a click opens an entry).
func (m *Model) commitSelected(e int) bool {
	return m.focused && m.IsCommitsTab() && e == m.commitCursor
}

// TabAt returns the preview tab under the mouse, or -1.
func (m *Model) TabAt(msg tea.MouseMsg) int { return m.carousel.ItemAt(msg) }

// SelectTab shows tab i.
func (m *Model) SelectTab(i int) { m.carousel.SetCursor(i) }

const commitZonePrefix = "pr-commit-"

func markCommit(i int, rendered string) string {
	if zone.DefaultManager == nil { // not set up (tests)
		return rendered
	}
	return zone.Mark(commitZonePrefix+strconv.Itoa(i), rendered)
}

// CommitAt returns the Commits tab entry under the mouse, or -1.
func (m *Model) CommitAt(msg tea.MouseMsg) int {
	if zone.DefaultManager == nil || !m.showsSection("Commits") {
		return -1
	}
	for i := range m.numCommitEntries() {
		if zone.Get(commitZonePrefix + strconv.Itoa(i)).InBounds(msg) {
			return i
		}
	}
	return -1
}

// SelectCommit puts the cursor on entry i.
func (m *Model) SelectCommit(i int) { m.MoveCommitCursor(i - m.commitCursor) }
