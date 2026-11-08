package prssection

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
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
		got = append(got, fmt.Sprintf("%d:%d", pr.Primary.Number, pr.StackDepth))
	}
	require.Equal(t, []string{
		"21:0",
		"5:0",
		"13:0", "22:1", "23:2",
		"8:0", "7:1",
		"31:1", "30:1", // each stops at the other: no endless indent
	}, got)
}
