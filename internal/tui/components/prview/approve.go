package prview

import (
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// The approve button at the top of the preview. Clicking it, or pressing
// enter on it in the "arrows" navigation, does what the Approve key (v) does.

// ApproveZoneId is the bubblezone id of the approve button.
const ApproveZoneId = "pr-approve"

type approveState int

const (
	approveHidden   approveState = iota // no PR, own PR, merged or closed
	approveEnabled                      // can be approved
	approveApproved                     // the current user's latest review approves it
)

func (m *Model) approveState() approveState {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return approveHidden
	}
	pr := m.pr.Data.Primary
	if pr.State != "OPEN" {
		return approveHidden
	}
	if m.ctx.User != "" && pr.Author.Login == m.ctx.User {
		return approveHidden
	}
	if m.ctx.User != "" && m.pr.Data.IsEnriched {
		latest := ""
		for _, r := range m.pr.Data.Enriched.Reviews.Nodes {
			if r.Author.Login != m.ctx.User || r.State == "COMMENTED" || r.State == "PENDING" {
				continue
			}
			latest = r.State // reviews are in chronological order
		}
		if latest == "APPROVED" {
			return approveApproved
		}
	}
	return approveEnabled
}

// CanApprove reports whether the approve button is shown and enabled.
func (m *Model) CanApprove() bool {
	return m.approveState() == approveEnabled
}

func (m *Model) SetApproveFocused(focused bool) {
	m.approveFocused = focused
	m.carousel.SetHighlighted(m.focused && !focused)
}

func (m *Model) IsApproveFocused() bool {
	return m.approveFocused
}

func (m *Model) renderApproveButton() string {
	switch m.approveState() {
	case approveEnabled:
		style := lipgloss.NewStyle().
			Bold(true).
			Background(m.ctx.Theme.SuccessText).
			Foreground(lipgloss.ANSIColor(0))
		if m.focused && m.approveFocused {
			style = style.Reverse(true)
		}
		button := style.Render(" ✓ Approve ")
		if zone.DefaultManager == nil { // not set up (tests)
			return button
		}
		return zone.Mark(ApproveZoneId, button)
	case approveApproved:
		return lipgloss.NewStyle().
			Foreground(m.ctx.Theme.FaintText).
			Render(" ✓ Approved ")
	}
	return ""
}
