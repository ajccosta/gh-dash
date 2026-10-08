package prview

import (
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// The merge button, next to the approve button. Clicking it, or pressing
// enter on it in the "arrows" navigation, does what the Merge key (m) does:
// it asks "Are you sure you want to merge this PR? (y/N)" and then runs
// `gh pr merge`.

// MergeZoneId is the bubblezone id of the merge button.
const MergeZoneId = "pr-merge"

// MergeBlockedReason is why GitHub would refuse the merge, from the PR data
// already loaded (mergeable, reviewDecision), or "" if nothing says so.
func (m *Model) MergeBlockedReason() string {
	if !m.CanMerge() {
		return ""
	}
	pr := m.pr.Data.Primary
	switch {
	case pr.Mergeable == "CONFLICTING":
		return "conflicts"
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return "changes requested"
	case pr.ReviewDecision == "REVIEW_REQUIRED":
		return "review required"
	}
	return ""
}

// CanMerge reports whether the merge button is shown: on open, non-draft PRs,
// your own included. A blocked merge is still shown (dimmed), as admins can
// bypass the rules and gh says why it fails.
func (m *Model) CanMerge() bool {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return false
	}
	pr := m.pr.Data.Primary
	return pr.State == "OPEN" && !pr.IsDraft
}

func (m *Model) SetMergeFocused(focused bool) {
	m.mergeFocused = focused
	m.carousel.SetHighlighted(m.focused && !m.approveFocused && !focused)
}

func (m *Model) IsMergeFocused() bool {
	return m.mergeFocused
}

func (m *Model) renderMergeButton() string {
	if !m.CanMerge() {
		return ""
	}
	style := lipgloss.NewStyle().Bold(true)
	label := " ⇣ Merge "
	if reason := m.MergeBlockedReason(); reason != "" {
		label = " Merge (" + reason + ") "
		style = style.Bold(false).Foreground(m.ctx.Theme.FaintText)
	} else {
		style = style.
			Background(m.ctx.Styles.Colors.MergedPR.Dark).
			Foreground(lipgloss.ANSIColor(0))
	}
	if m.focused && m.mergeFocused {
		style = style.Reverse(true)
	}
	button := style.Render(label)
	if zone.DefaultManager == nil { // not set up (tests)
		return button
	}
	return zone.Mark(MergeZoneId, button)
}
