package prview

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	gitmerge "github.com/dlvhdr/gh-dash/v4/internal/conflicts"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// The PR's conflicts with the other open PRs into its base, on the Overview
// and the Conflicts tab: which of them, once merged, it would conflict with,
// and in which files. GitHub is not asked: the other PRs are those the
// dashboard lists, and git, in the repo's local checkout, fetches them in one
// go and merges in memory. Only PRs that change a file this one changes are
// merged: each into the base, then this one into that.

// peerConflict is an open PR the shown one conflicts with once it is merged.
type peerConflict = gitmerge.Peer

type peerConflicts struct {
	pending bool
	checked int // the other PRs looked at
	found   []peerConflict
	err     error
}

// peerCache is what git found, by the shown PR and the other PRs, each as it
// was (conflictKey).
var peerCache = map[string]peerConflicts{}

// peers are the open PRs of the shown PR's repo into its base, but it.
func (m *Model) peers(open []*data.PullRequestData) []*data.PullRequestData {
	p := m.pr.Data.Primary
	var peers []*data.PullRequestData
	for _, q := range open {
		if q != nil && q.State == "OPEN" && q.Number != p.Number && q.BaseRefName == p.BaseRefName &&
			q.Repository.NameWithOwner == p.Repository.NameWithOwner &&
			!slices.ContainsFunc(peers, func(o *data.PullRequestData) bool { return o.Number == q.Number }) {
			peers = append(peers, q)
		}
	}
	slices.SortFunc(peers, func(a, b *data.PullRequestData) int { return a.Number - b.Number })
	return peers
}

// findPeerConflicts has git check the shown PR against the open PRs, if
// nobody has yet.
func (m *Model) findPeerConflicts(open []*data.PullRequestData) tea.Cmd {
	m.peerKey = ""
	p := m.pr.Data.Primary
	if p.State != "OPEN" {
		return nil
	}
	peers := m.peers(open)
	if len(peers) == 0 {
		return nil
	}
	key := m.conflictKey()
	numbers := make([]int, len(peers))
	for i, q := range peers {
		key += fmt.Sprintf(" #%d@%d", q.Number, q.UpdatedAt.UnixNano())
		numbers[i] = q.Number
	}
	m.peerKey = key
	conflictsMu.Lock()
	defer conflictsMu.Unlock()
	if _, ok := peerCache[key]; ok {
		return nil
	}
	repo := p.Repository.NameWithOwner
	dir, ok := common.GetRepoLocalPath(repo, m.ctx.Config.RepoPaths)
	if !ok {
		peerCache[key] = peerConflicts{err: fmt.Errorf("add %s to repoPaths to check", repo)}
		return nil
	}
	peerCache[key] = peerConflicts{pending: true}
	number, base := p.Number, p.BaseRefName
	return func() tea.Msg {
		found, err := peerConflictingFiles(dir, repo, number, base, numbers)
		conflictsMu.Lock()
		peerCache[key] = peerConflicts{checked: len(numbers), found: found, err: err}
		conflictsMu.Unlock()
		return ConflictsMsg{}
	}
}

// peerConflictingFiles finds which PRs of peers PR number of repo conflicts
// with, once that PR is merged into base. The conflict markers name the two
// sides gh-dash/peers/<number>/<base>+pr-<peer> and gh-dash/peers/<number>/pr-<number>.
func peerConflictingFiles(dir, repo string, number int, base string, peers []int) ([]peerConflict, error) {
	found, err := gitmerge.WithPeers(dir, repo, base, fmt.Sprintf("peers/%d", number), []int{number}, peers)
	return found[number], err
}

// shownPeerConflicts is what git found for the shown PR and the open PRs;
// checked 0 when there are none.
func (m *Model) shownPeerConflicts() peerConflicts {
	if m.peerKey == "" || m.pr == nil || m.pr.Data.Primary == nil || m.pr.Data.Primary.State != "OPEN" {
		return peerConflicts{}
	}
	conflictsMu.Lock()
	defer conflictsMu.Unlock()
	c, ok := peerCache[m.peerKey]
	if !ok {
		c.pending = true
	}
	return c
}

// renderPeerConflicts says which open PRs this one conflicts with, each
// one's files to open, entries from e on; brief leaves it out unless there are
// some.
func (m *Model) renderPeerConflicts(e int, brief bool) string {
	c := m.shownPeerConflicts()
	if c.checked == 0 && !c.pending && c.err == nil || brief && len(c.found) == 0 {
		return ""
	}
	base := m.pr.Data.Primary.BaseRefName
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2)
	switch {
	case c.pending:
		return lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.WaitingGlyph, " ",
			"Conflicts with open PRs into "+base) + "\n" + faint.Render("Checking with git…")
	case c.err != nil:
		return lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.FailureGlyph, " ",
			"Conflicts with open PRs into "+base) + "\n" + faint.Render(c.err.Error())
	case len(c.found) == 0:
		return lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.SuccessGlyph, " ",
			m.ctx.Styles.Common.FaintTextStyle.Render(fmt.Sprintf("No conflicts with the %d other open PRs into %s",
				c.checked, base)))
	}
	lines := []string{lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.FailureGlyph, " ",
		fmt.Sprintf("Conflicts with %d of the %d other open PRs into %s", len(c.found), c.checked, base))}
	for _, q := range c.found {
		lines = append(lines, faint.Render(fmt.Sprintf("once #%d %s is merged:", q.Number, m.peerTitle(q.Number))))
		lines = append(lines, m.conflictGroup(&e, q.Files)...)
	}
	return strings.Join(lines, "\n")
}

// peerTitle is the title of open PR number, as the dashboard lists it.
func (m *Model) peerTitle(number int) string {
	for _, q := range m.openPRs {
		if q.Number == number {
			return q.Title
		}
	}
	return ""
}
