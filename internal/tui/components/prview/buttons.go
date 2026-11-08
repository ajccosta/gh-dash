package prview

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
)

// The button row at the top of the preview, right of the branches. Each
// button does what its key does: the UI presses it on click and, in the
// "arrows" navigation, on enter, and the footer hints at its key while it can
// be pressed. A new button is one entry in buttons.

type button struct {
	zoneId  string
	action  string       // what the UI runs: "approve", else a confirmation prompt's action
	key     *key.Binding // the key that does the same
	enabled func(*Model) bool
	render  func(*Model) string // "" when hidden
}

// buttons is in row order, left to right.
var buttons = []button{
	{ApproveZoneId, "approve", &keys.PRKeys.Approve, (*Model).CanApprove, (*Model).renderApproveButton},
	{MergeZoneId, "merge", &keys.PRKeys.Merge, (*Model).CanMerge, (*Model).renderMergeButton},
	{CloseZoneId, "close", &keys.PRKeys.Close, (*Model).CanClose, (*Model).renderCloseButton},
	{DeleteBranchZoneId, "deleteBranch", &keys.PRKeys.DeleteBranch, (*Model).CanDeleteBranch, (*Model).renderDeleteBranchButton},
}

// ButtonIds are the zone ids of the buttons that can be pressed now, in row order.
func (m *Model) ButtonIds() []string {
	var ids []string
	for _, b := range buttons {
		if b.enabled(m) {
			ids = append(ids, b.zoneId)
		}
	}
	return ids
}

// ButtonAction is the action of the button with this zone id, or "" if it
// can't be pressed now.
func (m *Model) ButtonAction(id string) string {
	for _, b := range buttons {
		if b.zoneId == id && b.enabled(m) {
			return b.action
		}
	}
	return ""
}

// ButtonHints are the footer hints ("v approve") of the buttons that can be
// pressed now.
func (m *Model) ButtonHints() []string {
	var hints []string
	for _, b := range buttons {
		if b.enabled(m) {
			hints = append(hints, b.key.Help().Key+" "+b.key.Help().Desc)
		}
	}
	return hints
}

// SetFocusedButton gives focus to the button with this zone id, or to none
// with "" (the tabs bar is then highlighted if the preview has focus).
func (m *Model) SetFocusedButton(id string) {
	if !slices.Contains(m.ButtonIds(), id) {
		id = ""
	}
	m.focusedButton = id
	m.carousel.SetHighlighted(m.focused && id == "")
}

func (m *Model) FocusedButton() string {
	return m.focusedButton
}

func (m *Model) isButtonFocused(id string) bool {
	return m.focused && m.focusedButton == id
}

func (m *Model) renderButtons() string {
	var rendered []string
	for _, b := range buttons {
		if r := b.render(m); r != "" {
			rendered = append(rendered, r)
		}
	}
	return strings.Join(rendered, " ")
}
