package prview

import (
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// The close button, right of the merge button. Clicking it, or pressing enter
// on it in the "arrows" navigation, does what the Close key (x) does: it asks
// "Are you sure you want to close this PR? (y/N)" and then runs `gh pr close`.

// CloseZoneId is the bubblezone id of the close button.
const CloseZoneId = "pr-close"

// CanClose reports whether the close button is shown: on open PRs, drafts and
// your own included.
func (m *Model) CanClose() bool {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return false
	}
	return m.pr.Data.Primary.State == "OPEN"
}

func (m *Model) renderCloseButton() string {
	if !m.CanClose() {
		return ""
	}
	style := lipgloss.NewStyle().
		Bold(true).
		Background(m.ctx.Theme.ErrorText).
		Foreground(lipgloss.ANSIColor(0))
	if m.isButtonFocused(CloseZoneId) {
		style = style.Reverse(true)
	}
	button := style.Render(" ✕ Close ")
	if zone.DefaultManager == nil { // not set up (tests)
		return button
	}
	return zone.Mark(CloseZoneId, button)
}
