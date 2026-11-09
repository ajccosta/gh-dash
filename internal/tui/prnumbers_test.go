package tui

import (
	"testing"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"

	"github.com/stretchr/testify/require"
)

// commandFor is the command a custom PR key with this template runs on the
// current row.
func commandFor(t *testing.T, m *Model, tmpl string) string {
	m.formProgress = &startProgressMsg{}
	defer func() { m.formProgress = nil }()
	cmd := m.runCustomPRCommand(tmpl, m.getCurrRowData().(*prrow.Data))
	require.NotNil(t, cmd)
	msg, ok := cmd().(startProgressMsg)
	require.True(t, ok, "no command run")
	return msg.command
}

func TestPrNumbers_TheSelectedPRs(t *testing.T) {
	m, sec, shown := newBatchTestModel(t)
	tmpl := "open {{.RepoName}} {{.PrNumbers}}"
	require.Equal(t, "open o/r 2", commandFor(t, m, tmpl), "none selected: the current PR")

	sec.ToggleMarked(1)
	sec.ToggleMarked(2)
	require.Equal(t, "open o/r 1 3", commandFor(t, m, tmpl))
	require.Equal(t, "one 2", commandFor(t, m, "one {{.PrNumber}}"), "PrNumber stays the current PR")

	sec.Unmark(nil)
	sec.Prs[2].Primary.Repository.NameWithOwner = "o/other"
	sec.ToggleMarked(1)
	sec.ToggleMarked(2)
	m.formProgress = &startProgressMsg{}
	m.runCustomPRCommand(tmpl, &sec.Prs[0])
	m.formProgress = nil
	require.Contains(t, *shown, "The selected PRs are in several repos")
}
