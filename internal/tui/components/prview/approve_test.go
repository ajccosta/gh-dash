package prview

import (
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
