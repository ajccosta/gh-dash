package prview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

// The approve button at the top of the preview. Clicking it, or pressing
// enter on it in the "arrows" navigation, does what the Approve key (v) does.
// GitHub won't let you approve your own PR, so on yours it approves it
// locally (data.SetLocalApproval): it shows green until new commits come.

// ApproveZoneId is the bubblezone id of the approve button.
const ApproveZoneId = "pr-approve"

type approveState int

const (
	approveHidden   approveState = iota // no PR, merged or closed; own PR with no head commit known
	approveEnabled                      // can be approved
	approveApproved                     // the current user's latest review approves it
	approveLocal                        // own PR: can be approved locally
	approvedLocal                       // own PR, approved locally at its head: pressing takes it back
)

func (m *Model) approveState() approveState {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return approveHidden
	}
	pr := m.pr.Data.Primary
	if pr.State != "OPEN" {
		return approveHidden
	}
	if m.IsOwnPR() {
		if pr.HeadRefOid == "" { // no head commit to approve it at
			return approveHidden
		}
		if pr.IsLocallyApproved() {
			return approvedLocal
		}
		return approveLocal
	}
	if m.ctx.User != "" && m.pr.Data.IsEnriched {
		latest := ""
		for _, r := range m.pr.Data.Enriched.Reviews.Nodes {
			if !strings.EqualFold(r.Author.Login, m.ctx.User) || r.State == "COMMENTED" || r.State == "PENDING" {
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

// IsOwnPR reports whether the current gh user opened the previewed PR (GitHub
// rejects approving your own PR). GitHub's viewerDidAuthor, fetched with the
// PR, decides it; the login comparison (logins are case-insensitive) covers
// data without that field.
func (m *Model) IsOwnPR() bool {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return false
	}
	pr := m.pr.Data.Primary
	if pr.ViewerDidAuthor || (m.pr.Data.IsEnriched && m.pr.Data.Enriched.ViewerDidAuthor) {
		return true
	}
	return m.ctx.User != "" && strings.EqualFold(pr.Author.Login, m.ctx.User)
}

// CanApprove reports whether the approve button is shown and enabled.
func (m *Model) CanApprove() bool {
	s := m.approveState()
	return s == approveEnabled || s == approveLocal || s == approvedLocal
}

// ToggleLocalApproval approves your own previewed PR locally, at its head,
// or takes that back; it says which, or "" when the PR isn't yours.
func (m *Model) ToggleLocalApproval() string {
	s := m.approveState()
	if s != approveLocal && s != approvedLocal {
		return ""
	}
	pr := m.pr.Data.Primary
	if s == approvedLocal {
		data.SetLocalApproval(pr.Repository.NameWithOwner, pr.Number, "")
		return fmt.Sprintf("#%d no longer approved", pr.Number)
	}
	data.SetLocalApproval(pr.Repository.NameWithOwner, pr.Number, pr.HeadRefOid)
	return fmt.Sprintf("#%d approved, here only (GitHub isn't told)", pr.Number)
}

func (m *Model) renderApproveButton() string {
	label := " ✓ Approve "
	style := lipgloss.NewStyle().
		Bold(true).
		Background(m.ctx.Theme.SuccessText).
		Foreground(lipgloss.ANSIColor(0))
	switch s := m.approveState(); s {
	case approveEnabled, approveLocal, approvedLocal:
		switch {
		case s == approvedLocal:
			label = " ✓ Approved here · undo "
			style = lipgloss.NewStyle().Bold(true).Foreground(m.ctx.Theme.SuccessText)
		case s == approveLocal && data.LocalApproval(m.pr.Data.Primary.Repository.NameWithOwner,
			m.pr.Data.Primary.Number) != "":
			label = " ✓ Approve here again (new commits) "
		case s == approveLocal:
			label = " ✓ Approve here "
		}
		if m.isButtonFocused(ApproveZoneId) {
			style = style.Reverse(true)
		}
		button := style.Render(label)
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

// PRNumber is the number of the previewed PR, or 0.
func (m *Model) PRNumber() int {
	if !m.hasData() || m.pr.Data.Primary == nil {
		return 0
	}
	return m.pr.Data.Primary.Number
}
