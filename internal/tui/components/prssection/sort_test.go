package prssection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func numbers(prs []prrow.Data) []int {
	var ns []int
	for _, pr := range prs {
		ns = append(ns, pr.Primary.Number)
	}
	return ns
}

// Sorting, and going back to GitHub's order, never fetches the list again.
func TestSortBy_SortsHere(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := newTestModel("")
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	m.Ctx.Config = &cfg
	m.Ctx.Theme = theme.ParseTheme(&cfg)
	m.Ctx.Styles = context.InitStyles(m.Ctx.Theme)
	m.Table = table.NewModel(*m.Ctx, constants.Dimensions{Width: 80, Height: 20}, time.Time{}, time.Time{},
		[]table.Column{stackColumn()}, nil, "PR", nil, "", false)
	m.Prs = nil
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i, n := range []int{1, 2, 3} { // GitHub's order; #2 is the newest, #3 the oldest
		pr := stackPR("o/r", n, "h", "main")
		pr.Primary.CreatedAt = day.AddDate(0, 0, []int{1, 2, 0}[i])
		pr.Primary.UpdatedAt = pr.Primary.CreatedAt
		pr.Order = i
		m.Prs = append(m.Prs, pr)
	}

	require.Nil(t, m.SortBy("created"))
	require.Equal(t, []int{2, 1, 3}, numbers(m.Prs))
	require.Nil(t, m.SortBy("created"))
	require.Equal(t, []int{3, 1, 2}, numbers(m.Prs), "reversed")
	require.Nil(t, m.SortBy("updated"))
	require.Equal(t, []int{2, 1, 3}, numbers(m.Prs))

	m.sort = "base"
	m.addStackParents(stackParentsMsg{prs: []data.PullRequestData{}})
	m.Prs = append(m.Prs, prrow.Data{Primary: stackPR("o/r", 9, "x", "main").Primary, Extra: true})
	require.Nil(t, m.ToggleStacks())
	require.Equal(t, []int{1, 2, 3}, numbers(m.Prs), "GitHub's order, without the stack's extra PRs")
	require.Nil(t, m.SortBy("base"), "the PRs stacked on are kept from last time")
}
