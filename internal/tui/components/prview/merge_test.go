package prview

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

func TestMergeButton(t *testing.T) {
	testCases := map[string]struct {
		state, mergeable, decision string
		draft                      bool
		own                        bool
		render                     string // "" = hidden
	}{
		"open PR":             {state: "OPEN", mergeable: "MERGEABLE", render: "⇣ Merge"},
		"own open PR":         {state: "OPEN", mergeable: "MERGEABLE", own: true, render: "⇣ Merge"},
		"draft":               {state: "OPEN", draft: true},
		"merged":              {state: "MERGED"},
		"closed":              {state: "CLOSED"},
		"conflicts":           {state: "OPEN", mergeable: "CONFLICTING", render: "Merge (conflicts)"},
		"review required":     {state: "OPEN", decision: "REVIEW_REQUIRED", render: "Merge (review required)"},
		"changes requested":   {state: "OPEN", decision: "CHANGES_REQUESTED", render: "Merge (changes requested)"},
		"approved, mergeable": {state: "OPEN", mergeable: "MERGEABLE", decision: "APPROVED", render: "⇣ Merge"},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pr := &data.PullRequestData{State: tc.state, Title: "t", IsDraft: tc.draft,
				Mergeable: tc.mergeable, ReviewDecision: tc.decision, ViewerDidAuthor: tc.own}
			pr.Author.Login = "other"
			m := newTestModelWithWidth(t, pr, nil, nil, 100)
			m.ctx.User = "me"
			require.Equal(t, tc.render != "", m.CanMerge())
			button := m.renderMergeButton()
			if tc.render == "" {
				require.Empty(t, button)
				require.NotContains(t, m.View(), "⇣ Merge")
				require.NotContains(t, m.View(), "Merge (")
				return
			}
			require.Contains(t, button, tc.render)
			require.Contains(t, m.View(), tc.render)
			if tc.own {
				require.NotContains(t, m.View(), "Approve", "own PR: Merge but no Approve")
			}
		})
	}
}
