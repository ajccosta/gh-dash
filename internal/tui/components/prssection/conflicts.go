package prssection

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/conflicts"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

// The Conflicts column: which open PRs of the list each one conflicts with,
// once they are merged. Git works it out, without asking GitHub, for the open
// PRs of a repo with a local checkout (repoPaths) into the same base: one
// fetch, then merges in memory of the PRs that change the same files. It is
// done again once one of them changes.

// conflictGroup is the open PRs of the list of one repo into one base.
type conflictGroup struct {
	repo, base, dir string
	numbers         []int
}

type groupConflicts struct {
	checking bool
	peers    map[int][]int
	err      error
}

var (
	groupConflictsMu sync.Mutex
	groupConflictsBy = map[string]groupConflicts{} // by the key conflictGroups gives
	groupJobs        atomic.Int64                  // each check's refs are its own
)

// conflictsCheckedMsg says git is done with a group.
type conflictsCheckedMsg struct{}

// prKey is a PR of the list.
type prKey struct {
	repo   string
	number int
}

// conflictGroups are the list's groups of two PRs or more, by a key that
// changes when one of them does, and the key of each PR's group.
func (m *Model) conflictGroups() (groups map[string]conflictGroup, of map[prKey]string) {
	type at struct{ repo, base string }
	members := map[at][]*prrow.Data{}
	var order []at
	for i := range m.Prs {
		p := m.Prs[i].Primary
		if p == nil || p.State != "OPEN" {
			continue
		}
		g := at{p.Repository.NameWithOwner, p.BaseRefName}
		if _, ok := members[g]; !ok {
			order = append(order, g)
		}
		if !slices.ContainsFunc(members[g], func(d *prrow.Data) bool { return d.Primary.Number == p.Number }) {
			members[g] = append(members[g], &m.Prs[i])
		}
	}
	groups, of = map[string]conflictGroup{}, map[prKey]string{}
	for _, g := range order {
		prs := members[g]
		if len(prs) < 2 {
			continue
		}
		dir, ok := common.GetRepoLocalPath(g.repo, m.Ctx.Config.RepoPaths)
		if !ok {
			continue
		}
		slices.SortFunc(prs, func(a, b *prrow.Data) int { return a.Primary.Number - b.Primary.Number })
		key := g.repo + " " + g.base
		var numbers []int
		for _, d := range prs {
			key += fmt.Sprintf(" #%d@%d", d.Primary.Number, d.Primary.UpdatedAt.UnixNano())
			numbers = append(numbers, d.Primary.Number)
		}
		groups[key] = conflictGroup{g.repo, g.base, dir, numbers}
		for _, n := range numbers {
			of[prKey{g.repo, n}] = key
		}
	}
	return groups, of
}

// checkConflicts has git check the groups nobody has yet.
func (m *Model) checkConflicts() tea.Cmd {
	groups, _ := m.conflictGroups()
	groupConflictsMu.Lock()
	defer groupConflictsMu.Unlock()
	var cmds []tea.Cmd
	for key, g := range groups {
		if _, ok := groupConflictsBy[key]; ok {
			continue
		}
		groupConflictsBy[key] = groupConflicts{checking: true}
		ns := fmt.Sprintf("list/%d", groupJobs.Add(1))
		cmds = append(cmds, m.MakeSectionCmd(func() tea.Msg {
			found, err := conflicts.WithPeers(g.dir, g.repo, g.base, ns, g.numbers, g.numbers)
			peers := map[int][]int{}
			for n, ps := range found {
				for _, p := range ps {
					peers[n] = append(peers[n], p.Number)
				}
				slices.Sort(peers[n])
			}
			groupConflictsMu.Lock()
			groupConflictsBy[key] = groupConflicts{peers: peers, err: err}
			groupConflictsMu.Unlock()
			return conflictsCheckedMsg{}
		}))
	}
	return tea.Batch(cmds...)
}

// rowConflicts is what git found for each PR of the list.
func (m *Model) rowConflicts() map[prKey]prrow.Conflicts {
	_, of := m.conflictGroups()
	groupConflictsMu.Lock()
	defer groupConflictsMu.Unlock()
	rows := map[prKey]prrow.Conflicts{}
	for pr, key := range of {
		if g, ok := groupConflictsBy[key]; ok {
			rows[pr] = prrow.Conflicts{Checking: g.checking, Peers: g.peers[pr.number], Err: g.err}
		}
	}
	return rows
}
