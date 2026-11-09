package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// stubMerge puts a gh on PATH that logs its arguments and fails `gh pr merge
// fail`.
func stubMerge(t *testing.T, fail string) string {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	script := `#!/bin/sh
echo "$*" >> '` + log + `'
[ "$1 $2 $3" = "pr merge ` + fail + `" ] && { echo "not mergeable" >&2; exit 1; }
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// newBatchTestModel lists, in o/r, #2 (stacked on #1), #1 and #3, and records
// the notifications shown.
func newBatchTestModel(t *testing.T) (*Model, *prssection.Model, *[]string) {
	zone.NewGlobal()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	data.SetFormChoices("merge", map[string]string{"method": "merge", "delete": "no"})
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	var shown []string
	m.ctx.StartTask = func(task context.Task) tea.Cmd {
		shown = append(shown, task.StartText)
		return nil
	}
	sec := prssection.NewModel(0, m.ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	for _, p := range []struct {
		n          int
		head, base string
	}{{2, "b", "a"}, {1, "a", "main"}, {3, "c", "main"}} {
		pr := &data.PullRequestData{Number: p.n, Title: "PR", State: "OPEN",
			HeadRefName: p.head, BaseRefName: p.base}
		pr.Repository.NameWithOwner = "o/r"
		sec.Prs = append(sec.Prs, prrow.Data{Primary: pr})
	}
	m.prs = []section.Section{&sec}
	return &m, &sec, &shown
}

// runBatch merges what is selected, as the dialog's defaults say, running
// each merge as the dashboard would.
func runBatch(t *testing.T, m *Model, sec *prssection.Model) {
	require.Nil(t, m.promptConfirmation(sec, "merge"))
	cmd := m.dialog.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for m.batch != nil {
		retries := m.batch.retries
		cmd = m.batchMergeStep(finished(t, cmd))
		if m.batch != nil && m.batch.retries > retries {
			cmd = m.batchMergeNext() // a retry, without waiting for its tick
		}
	}
}

func TestBatchMerge_ParentsFirstOneAtATime(t *testing.T) {
	log := stubMerge(t, "")
	m, sec, shown := newBatchTestModel(t)
	sec.ToggleMarked(0)
	sec.ToggleMarked(1)
	sec.ToggleMarked(2)
	sec.ToggleMarked(2) // unselected again
	require.Len(t, sec.Marked(), 2)

	require.Nil(t, m.promptConfirmation(sec, "merge"))
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "Merge 2 PRs?")
	require.Contains(t, view, "Stacked: [#2]")
	m.dialog.Close()

	runBatch(t, m, sec)
	require.Equal(t, []string{"pr merge 1 -R o/r --merge", "pr merge 2 -R o/r --merge"}, ghCalls(t, log))
	require.Empty(t, sec.Marked())
	require.Contains(t, *shown, "Merged 2 PRs")
}

func TestBatchMerge_StopsAtTheFirstFailure(t *testing.T) {
	log := stubMerge(t, "1")
	m, sec, shown := newBatchTestModel(t)
	for i := range sec.Prs {
		sec.ToggleMarked(i)
	}
	runBatch(t, m, sec)
	calls := ghCalls(t, log)
	require.Len(t, calls, 1+batchRetries, "tried again, then stopped")
	for _, c := range calls {
		require.Equal(t, "pr merge 1 -R o/r --merge", c)
	}
	require.Len(t, sec.Marked(), 3, "all still selected")
	require.Contains(t, *shown, "Batch merge stopped at #1: 0 merged, 3 left selected")
}
