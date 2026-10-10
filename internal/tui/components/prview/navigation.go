package prview

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
	m.carousel.SetHighlighted(focused && !m.approveFocused && !m.mergeFocused)
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
	return m.carousel.SelectedItem() == tabs[2]
}

func (m *Model) numCommits() int {
	if m.pr == nil || !m.pr.Data.IsEnriched {
		return 0
	}
	return len(m.pr.Data.Enriched.AllCommits.Nodes)
}

// MoveCommitCursor moves the selected commit by delta, staying in range.
func (m *Model) MoveCommitCursor(delta int) {
	n := m.numCommits()
	if n == 0 {
		m.commitCursor = 0
		return
	}
	m.commitCursor = min(max(m.commitCursor+delta, 0), n-1)
}

// SelectedCommit returns the full hash of the selected commit and the
// repository it belongs to, or empty strings if there is none.
func (m *Model) SelectedCommit() (oid string, repo string, prNumber int) {
	n := m.numCommits()
	if n == 0 || m.pr.Data.Primary == nil {
		return "", "", 0
	}
	i := min(m.commitCursor, n-1)
	return m.pr.Data.Enriched.AllCommits.Nodes[i].Commit.Oid,
		m.pr.Data.Primary.GetRepoNameWithOwner(), m.pr.Data.Primary.GetNumber()
}

func (m *Model) commitSelected(i int) bool {
	return m.focused && i == m.commitCursor
}
