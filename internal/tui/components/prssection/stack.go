package prssection

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

// Sorting by the Base column puts the PRs in stack order: each one right
// under the PR whose branch it is based on, indented one more step, so a
// stack reads from the PR on the default branch down to its top. PRs on a
// branch that is no PR of the list (main, mostly) start a stack; stacks keep
// the order the list had, and so do PRs on the same branch.

// ToggleStacks puts the list in stack order (as a click on Base does), or
// back in GitHub's order.
func (m *Model) ToggleStacks() tea.Cmd {
	if m.sort != "base" {
		return m.SortBy("base")
	}
	m.sort, m.sortDesc = "", false
	data.SetSectionSort(m.sortId(), data.SectionSort{})
	m.ResetRows()
	return tea.Batch(m.FetchNextPageSectionRows()...)
}

// stackOrder arranges prs into stacks and sets each one's StackDepth.
func stackOrder(prs []prrow.Data) {
	type node struct {
		pr   prrow.Data
		path []int // positions before arranging, from the bottom of its stack to itself
	}
	byHead := map[[2]string]int{}
	for i, pr := range prs {
		if p := pr.Primary; p != nil && !p.IsCrossRepository {
			byHead[[2]string{p.Repository.NameWithOwner, p.HeadRefName}] = i
		}
	}
	nodes := make([]node, len(prs))
	for i, pr := range prs {
		n := node{pr: pr, path: []int{i}}
		for p, seen := pr.Primary, map[int]bool{i: true}; p != nil; {
			j, ok := byHead[[2]string{p.Repository.NameWithOwner, p.BaseRefName}]
			if !ok || seen[j] { // a cycle: start the stack here
				break
			}
			seen[j] = true
			n.path = append([]int{j}, n.path...)
			p = prs[j].Primary
		}
		n.pr.Stacked, n.pr.StackDepth = true, len(n.path)-1
		nodes[i] = n
	}
	// By the path: the stack's bottom PR keeps the stack's place in the list,
	// and a PR comes right before those stacked on it.
	slices.SortFunc(nodes, func(a, b node) int { return slices.Compare(a.path, b.path) })
	for i, n := range nodes {
		prs[i] = n.pr
	}
}
