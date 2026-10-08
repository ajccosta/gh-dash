package prview

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

func TestApproveState(t *testing.T) {
	review := func(login, state string) data.Review {
		r := data.Review{State: state}
		r.Author.Login = login
		return r
	}
	testCases := map[string]struct {
		state   string
		author  string
		reviews []data.Review
		want    approveState
		render  string
	}{
		"open PR by someone else":  {state: "OPEN", author: "other", want: approveEnabled, render: "✓ Approve"},
		"own PR":                   {state: "OPEN", author: "me", want: approveHidden},
		"merged PR":                {state: "MERGED", author: "other", want: approveHidden},
		"closed PR":                {state: "CLOSED", author: "other", want: approveHidden},
		"already approved":         {state: "OPEN", author: "other", reviews: []data.Review{review("me", "APPROVED")}, want: approveApproved, render: "✓ Approved"},
		"approved then commented":  {state: "OPEN", author: "other", reviews: []data.Review{review("me", "APPROVED"), review("me", "COMMENTED")}, want: approveApproved, render: "✓ Approved"},
		"approval dismissed":       {state: "OPEN", author: "other", reviews: []data.Review{review("me", "APPROVED"), review("me", "DISMISSED")}, want: approveEnabled, render: "✓ Approve"},
		"changes requested by me":  {state: "OPEN", author: "other", reviews: []data.Review{review("me", "CHANGES_REQUESTED")}, want: approveEnabled, render: "✓ Approve"},
		"approved by someone else": {state: "OPEN", author: "other", reviews: []data.Review{review("third", "APPROVED")}, want: approveEnabled, render: "✓ Approve"},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pr := &data.PullRequestData{State: tc.state, Title: "t"}
			pr.Author.Login = tc.author
			m := newTestModelWithWidth(t, pr, tc.reviews, nil, 80)
			m.ctx.User = "me"
			require.Equal(t, tc.want, m.approveState())
			require.Equal(t, tc.want == approveEnabled, m.CanApprove())
			button := m.renderApproveButton()
			if tc.render == "" {
				require.Empty(t, button)
			} else {
				require.Contains(t, button, tc.render)
				require.Contains(t, m.View(), tc.render)
			}
		})
	}
}

// ajccosta/deqalloc-private#4 as the search query returns it (trimmed): the
// user's own PR, which GitHub refuses to approve.
const ownPRJSON = `{
  "number": 4,
  "title": "Generic StatCounter",
  "author": {"login": "ajccosta"},
  "authorAssociation": "OWNER",
  "viewerDidAuthor": true,
  "state": "OPEN",
  "isDraft": false,
  "mergeable": "MERGEABLE",
  "reviewDecision": "",
  "repository": {"nameWithOwner": "ajccosta/deqalloc-private"}
}`

func TestApproveHiddenOnOwnPR(t *testing.T) {
	testCases := map[string]struct {
		user           string // ctx.User; "" = not fetched (yet)
		login          string
		viewerAuthored bool
		own            bool
	}{
		"viewerDidAuthor, user not loaded yet": {user: "", login: "ajccosta", viewerAuthored: true, own: true},
		"viewerDidAuthor and same login":       {user: "ajccosta", login: "ajccosta", viewerAuthored: true, own: true},
		"login differs only in case":           {user: "AJCcosta", login: "ajccosta", own: true},
		"someone else's PR":                    {user: "ajccosta", login: "dlvhdr", own: false},
		"someone else's, user not loaded":      {user: "", login: "dlvhdr", own: false},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var pr data.PullRequestData
			require.NoError(t, json.Unmarshal([]byte(ownPRJSON), &pr))
			require.True(t, pr.ViewerDidAuthor, "viewerDidAuthor decodes into the PR data")
			pr.Author.Login = tc.login
			pr.ViewerDidAuthor = tc.viewerAuthored
			m := newTestModelWithWidth(t, &pr, nil, nil, 100)
			m.ctx.User = tc.user
			require.Equal(t, tc.own, m.IsOwnPR())
			require.Equal(t, !tc.own, m.CanApprove())
			if tc.own {
				require.NotContains(t, m.View(), "Approve")
			} else {
				require.Contains(t, m.View(), "✓ Approve")
			}
		})
	}
}
