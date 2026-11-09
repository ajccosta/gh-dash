package prssection

import (
	"fmt"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

// Selecting PRs for a batch action (* or a modifier-click on a row): the
// selection outlives refetches and re-sorts, as it is kept by repo and number.

func markKey(pr *prrow.Data) string {
	if pr == nil || pr.Primary == nil {
		return ""
	}
	return fmt.Sprintf("%s#%d", pr.Primary.Repository.NameWithOwner, pr.Primary.Number)
}

// ToggleMarked selects the PR on row i, or unselects it.
func (m *Model) ToggleMarked(i int) {
	if i < 0 || i >= len(m.Prs) || m.Prs[i].Primary == nil {
		return
	}
	k := markKey(&m.Prs[i])
	if m.marked == nil {
		m.marked = map[string]bool{}
	}
	if m.marked[k] {
		delete(m.marked, k)
	} else {
		m.marked[k] = true
	}
	m.Table.SetRows(m.BuildRows())
}

// Marked is the selected PRs still in the list, in list order.
func (m *Model) Marked() []*prrow.Data {
	var prs []*prrow.Data
	for i := range m.Prs {
		if m.marked[markKey(&m.Prs[i])] {
			prs = append(prs, &m.Prs[i])
		}
	}
	return prs
}

// Unmark unselects one PR, or all of them with pr nil.
func (m *Model) Unmark(pr *prrow.Data) {
	if len(m.marked) == 0 {
		return
	}
	if pr == nil {
		m.marked = nil
	} else {
		delete(m.marked, markKey(pr))
	}
	m.Table.SetRows(m.BuildRows())
}
