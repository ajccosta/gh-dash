package prssection

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
	checks "github.com/dlvhdr/x/gh-checks"
)

// Clicking a column header sorts the PRs by it; clicking it again reverses.
// GitHub sorts the columns its search can sort by, so the order holds across
// pages; the others order the PRs loaded so far, and each page as it comes.

type prSort struct {
	desc      bool   // the first click puts the most, newest or best first
	qualifier string // GitHub's "sort:" for it, if GitHub can sort by it
	cmp       func(a, b *data.PullRequestData) int
}

var prSorts = map[string]prSort{
	"title":    {cmp: byText(func(p *data.PullRequestData) string { return p.Title })},
	"repo":     {cmp: byText(func(p *data.PullRequestData) string { return p.Repository.NameWithOwner })},
	"author":   {cmp: byText(func(p *data.PullRequestData) string { return p.Author.Login })},
	"base":     {cmp: byText(func(p *data.PullRequestData) string { return p.BaseRefName })},
	"comments": {desc: true, cmp: byNum(func(p *data.PullRequestData) int { return p.Comments.TotalCount + p.ReviewThreads.TotalCount })},
	"review":   {desc: true, cmp: byNum(reviewRank)},
	"checks":   {desc: true, cmp: byNum(checksRank)},
	"lines":    {desc: true, cmp: byNum(func(p *data.PullRequestData) int { return p.Additions + p.Deletions })},
	"updated":  {desc: true, qualifier: "updated"},
	"created":  {desc: true, qualifier: "created"},
}

func byText(f func(*data.PullRequestData) string) func(a, b *data.PullRequestData) int {
	return func(a, b *data.PullRequestData) int {
		return cmp.Compare(strings.ToLower(f(a)), strings.ToLower(f(b)))
	}
}

func byNum(f func(*data.PullRequestData) int) func(a, b *data.PullRequestData) int {
	return func(a, b *data.PullRequestData) int { return cmp.Compare(f(a), f(b)) }
}

// reviewRank: approved, reviewed, waiting, changes requested.
func reviewRank(p *data.PullRequestData) int {
	switch {
	case p.ReviewDecision == "APPROVED":
		return 3
	case p.ReviewDecision == "CHANGES_REQUESTED":
		return 0
	case p.Reviews.TotalCount > 0:
		return 2
	}
	return 1
}

// checksRank: passing, running, none, failing.
func checksRank(p *data.PullRequestData) int {
	switch (&prrow.PullRequest{Data: &prrow.Data{Primary: p}}).GetStatusChecksRollup() {
	case checks.CommitStateSuccess:
		return 3
	case checks.CommitStateExpected, checks.CommitStatePending:
		return 2
	case checks.CommitStateError, checks.CommitStateFailure:
		return 0
	}
	return 1
}

// sortable makes c sort by key and gives it the long title (unless
// theme.ui.table.shortHeaders), widened so the title and its arrow fit.
func sortable(ctx *context.ProgramContext, c table.Column, key, long string) table.Column {
	c.SortKey = key
	if !ctx.Config.Theme.Ui.Table.ShortHeaders {
		c.Title = long
	}
	if c.Width != nil {
		w := lipgloss.Width(c.Title + constants.SortDescIcon + "  ") // and the cell's padding
		c.Width = utils.IntPtr(max(*c.Width, w))
	}
	return c
}

// sortQualifiers are the "sort:" terms in a search.
var sortQualifiers = regexp.MustCompile(`(^|\s)sort:\S+`)

// GetFilters is the search, with GitHub's sort when it sorts the chosen column.
func (m *Model) GetFilters() string {
	f := m.BaseModel.GetFilters()
	s := prSorts[m.sort]
	if s.qualifier == "" {
		return f
	}
	dir := "-asc"
	if m.sortDesc {
		dir = "-desc"
	}
	return strings.TrimSpace(sortQualifiers.ReplaceAllString(f, "")) + " sort:" + s.qualifier + dir
}

// syncSortArrow puts the arrow on the column the list is in order of: the
// chosen one, else Updated while the search leaves GitHub's sort:updated.
func (m *Model) syncSortArrow() {
	m.Table.SortKey, m.Table.SortDesc = m.sort, m.sortDesc
	if m.sort == "" && !sortQualifiers.MatchString(m.BaseModel.GetFilters()) {
		m.Table.SortKey, m.Table.SortDesc = "updated", true
	}
}

// SortBy sorts by the column key, or reverses it if the list is already in
// its order. A change in what GitHub sorts by fetches the list again.
func (m *Model) SortBy(key string) tea.Cmd {
	s, ok := prSorts[key]
	if !ok {
		return nil
	}
	m.syncSortArrow()
	desc := s.desc
	if m.Table.SortKey == key {
		desc = !m.Table.SortDesc
	}
	before := m.GetFilters()
	m.sort, m.sortDesc = key, desc
	m.syncSortArrow()
	if m.GetFilters() != before {
		m.ResetRows()
		return tea.Batch(m.FetchNextPageSectionRows()...)
	}
	m.sortPrs()
	return nil
}

// sortPrs orders the loaded PRs by the chosen column, if GitHub can't, and
// keeps the selected PR selected.
func (m *Model) sortPrs() {
	s := prSorts[m.sort]
	if s.cmp == nil {
		return
	}
	var sel *data.PullRequestData
	if i := m.Table.GetCurrItem(); i >= 0 && i < len(m.Prs) {
		sel = m.Prs[i].Primary
	}
	slices.SortStableFunc(m.Prs, func(a, b prrow.Data) int {
		if a.Primary == nil || b.Primary == nil {
			return 0
		}
		if m.sortDesc {
			return s.cmp(b.Primary, a.Primary)
		}
		return s.cmp(a.Primary, b.Primary)
	})
	m.Table.SetRows(m.BuildRows())
	for i, pr := range m.Prs {
		if sel != nil && pr.Primary == sel {
			m.Table.SelectItem(i)
		}
	}
}
