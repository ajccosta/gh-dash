package prview

import (
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// The delete-branch button, where merge and close were, on a merged PR whose
// head branch is still there. Clicking it, or pressing enter on it in the
// "arrows" navigation, does what its key (B) does: a dialog lists the open PRs
// based on that branch, which are moved onto the merged PR's base before the
// branch is deleted.

// DeleteBranchZoneId is the bubblezone id of the delete-branch button.
const DeleteBranchZoneId = "pr-delete-branch"

// CanDeleteBranch reports whether the button is shown: on a merged PR whose
// head branch is in its own repo and not deleted yet.
func (m *Model) CanDeleteBranch() bool {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return false
	}
	pr := m.pr.Data.Primary
	return pr.State == "MERGED" && pr.HeadRef.Name != "" && !pr.IsCrossRepository
}

func (m *Model) renderDeleteBranchButton() string {
	if !m.CanDeleteBranch() {
		return ""
	}
	style := lipgloss.NewStyle().
		Bold(true).
		Background(m.ctx.Theme.ErrorText).
		Foreground(lipgloss.ANSIColor(0))
	if m.isButtonFocused(DeleteBranchZoneId) {
		style = style.Reverse(true)
	}
	button := style.Render(" ✕ Delete branch ")
	if zone.DefaultManager == nil { // not set up (tests)
		return button
	}
	return zone.Mark(DeleteBranchZoneId, button)
}
