package prssection

import (
	"fmt"
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

func stackPR(repo string, n int, head, base string) prrow.Data {
	p := &data.PullRequestData{Number: n, HeadRefName: head, BaseRefName: base}
	p.Repository.NameWithOwner = repo
	return prrow.Data{Primary: p}
}

func TestStackOrder(t *testing.T) {
	const a = "ajccosta/deqalloc-artifact"
	prs := []prrow.Data{ // as the search returned them
		stackPR(a, 23, "extended-allocators", "docker-and-progress"),
		stackPR(a, 21, "readme-results", "main"),
		stackPR(a, 7, "short-run", "zipfian"),
		stackPR(a, 22, "docker-and-progress", "bare-host-setup"),
		stackPR("o/other", 5, "x", "zipfian"), // another repo's zipfian
		stackPR(a, 13, "bare-host-setup", "main"),
		stackPR(a, 8, "zipfian", "main"),
		stackPR(a, 30, "a", "b"), // a cycle
		stackPR(a, 31, "b", "a"),
	}
	stackOrder(prs)
	var got []string
	for _, pr := range prs {
		got = append(got, fmt.Sprintf("%-6s#%d", pr.StackLanes[0], pr.Primary.Number))
	}
	// Top-down in merge order: each PR right under the one it is based on.
	require.Equal(t, []string{
		"├─    #21",
		"├─    #13",
		"│ └─  #22",
		"│   └─#23",
		"└─    #8",
		"  └─  #7",
		"└─    #5",  // on another repo's zipfian
		"└─    #30", // the cycle stops at 30: no endless indent
		"  └─  #31",
	}, got)
	require.Equal(t, "│   │", prs[2].StackLanes[1], "#22 has #23 below it")
	require.Equal(t, "│      ", prs[3].StackLanes[1], "#8 still to come")
}

func TestAddStackParents(t *testing.T) {
	const a = "o/r"
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
	m.sort = "base"
	stale := stackPR(a, 99, "old", "main")
	stale.Extra = true
	m.Prs = []prrow.Data{
		stackPR(a, 23, "extended", "docker"),
		stackPR(a, 22, "docker", "bare"),
		stale, // from an earlier fetch
	}
	fetched := func(n int, head, base string) data.PullRequestData {
		return *stackPR(a, n, head, base).Primary
	}
	m.addStackParents(stackParentsMsg{prs: []data.PullRequestData{
		fetched(22, "docker", "bare"),
		fetched(13, "bare", "x"),
		fetched(9, "x", "main"),
		fetched(50, "unrelated", "main"),
	}})
	var got []string
	for _, pr := range m.Prs {
		got = append(got, fmt.Sprintf("%-8s#%d %v", pr.StackLanes[0], pr.Primary.Number, pr.Extra))
	}
	require.Equal(t, []string{
		"└─      #9 true",
		"  └─    #13 true",
		"    └─  #22 false",
		"      └─#23 false",
	}, got)
	require.Equal(t, 11, *m.Table.Columns[0].Width, "the graph and the padding")
}
