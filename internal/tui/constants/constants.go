package constants

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	FirstItem     key.Binding
	LastItem      key.Binding
	TogglePreview key.Binding
	OpenGithub    key.Binding
	Refresh       key.Binding
	PageDown      key.Binding
	PageUp        key.Binding
	NextSection   key.Binding
	PrevSection   key.Binding
	Help          key.Binding
	Quit          key.Binding
}

type Dimensions struct {
	Width  int
	Height int
}

const (
	Ellipsis = "…"

	// Icons are plain characters that render in any monospace font (no Nerd Font).
	ApprovedIcon         = "✓"
	ChangesRequestedIcon = "±"
	DotIcon              = "●"
	SmallDotIcon         = "⋅"
	HorizontalLineIcon   = "─"
	EmptyIcon            = "-"
	FailureIcon          = "✗"
	PersonIcon           = "@"
	SuccessIcon          = "✓"
	TeamIcon             = "#"
	WaitingIcon          = "…"
	ActionRequiredIcon   = "!" // matches GitHub UI

	BehindIcon         = "↓"
	BlockedIcon        = "!"
	ClosedIcon         = "✗"
	CodeReviewIcon     = "@"
	CommentIcon        = "✎"
	CommentsIcon       = "✎"
	DonateIcon         = "♥"
	DraftIcon          = "○"
	CommitIcon         = "•"
	VerticalCommitIcon = "○"
	LabelsIcon         = "Labels"
	MergedIcon         = "◆"
	MergeQueueIcon     = "≡"
	OpenIcon           = "●"
	SelectionIcon      = "→"

	AutocompleteColumnGap              = 2
	AutocompleteMinValueWidth          = 8
	AutocompleteMinDetailWidth         = 10
	AutocompletePreferredValueRatioNum = 2
	AutocompletePreferredValueRatioDen = 3

	// New contributors: users who created a PR for the repo for the first time
	NewContributorIcon = "✦"

	// Contributors: everyone who has contributed something back to the project
	ContributorIcon = "+"

	// Collaborator is a person who isn't explicitly a member of your organization,
	// but who has Read, Write, or Admin permissions to one or more repositories in your organization.
	CollaboratorIcon = "◇"

	// A member of the organization
	MemberIcon = "◆"

	// The person/s who has administrative ownership over the organization or repository (not always the same as the original author)
	OwnerIcon = "★"

	UnknownRoleIcon = "?"

	// Notification type icons
	WorkflowIcon     = "▸" // for CheckSuite/CI
	WorkflowRunIcon  = "▸" // for CheckSuite default
	SecurityIcon     = "!" // for security alerts
	NotificationIcon = "•" // generic notification fallback
	SearchIcon       = "/"

	// Prompts
	AssignPrompt   = "Assign users (whitespace-separated)" + Ellipsis
	UnassignPrompt = "Unassign users (whitespace-separated)" + Ellipsis
	CommentPrompt  = "Leave a comment" + Ellipsis
	ApprovalPrompt = "Approve with comment" + Ellipsis
	LabelPrompt    = "Add/remove labels (comma-separated)" + Ellipsis

	Logo = `DASH`
)
