package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// scratch is a repo with an "origin" bare repo it pushes to, and a gh that
// says which PRs are based on a branch ($GH_STACKED) and knows no rules.
type scratch struct {
	t            *testing.T
	work, origin string
}

func newScratch(t *testing.T) *scratch {
	dir := t.TempDir()
	s := &scratch{t: t, work: filepath.Join(dir, "work"), origin: filepath.Join(dir, "origin.git")}
	for k, v := range map[string]string{
		"XDG_CACHE_HOME": filepath.Join(dir, "cache"), "GIT_CONFIG_GLOBAL": filepath.Join(dir, "gitconfig"),
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t",
	} {
		t.Setenv(k, v)
	}
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(`#!/bin/sh
case "$1 $2" in
"pr list") echo "$GH_STACKED" ;;
*) exit 1 ;;
esac
`), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_STACKED", "")
	old := cloneRepo
	cloneRepo = func(_, d string) error { _, err := runGit(".", "clone", "--quiet", s.origin, d); return err }
	t.Cleanup(func() { cloneRepo = old })
	s.git("init", "--quiet", "--bare", s.origin)
	s.git("init", "--quiet", "-b", "main", s.work)
	s.in("remote", "add", "origin", s.origin)
	return s
}

func (s *scratch) git(args ...string) string {
	out, err := exec.Command("git", args...).CombinedOutput()
	require.NoError(s.t, err, string(out))
	return strings.TrimSpace(string(out))
}

func (s *scratch) in(args ...string) string { return s.git(append([]string{"-C", s.work}, args...)...) }

// commit writes text to file and commits it; it returns the commit.
func (s *scratch) commit(file, text string) string {
	require.NoError(s.t, os.WriteFile(filepath.Join(s.work, file), []byte(text), 0o644))
	s.in("add", file)
	s.in("commit", "--quiet", "-m", "change "+file)
	return s.in("rev-parse", "HEAD")
}

func (s *scratch) push(branches ...string) {
	s.in(append([]string{"push", "--quiet", "--force", "origin"}, branches...)...)
}

func (s *scratch) plan(sha string) removeCommitPlanMsg {
	pr := &data.PullRequestData{Number: 10, Title: "Feature", State: "OPEN", HeadRefName: "feat", BaseRefName: "main"}
	pr.Repository.NameWithOwner = "o/r"
	p := removeCommitPlanMsg{pr: &prrow.Data{Primary: pr}, repo: "o/r", branch: "feat", base: "main",
		sha: sha, headline: "change"}
	p.try()
	require.NoError(s.t, p.err)
	return p
}

func (s *scratch) files(ref string) string {
	return s.git("-C", s.origin, "ls-tree", "--name-only", ref)
}

// linear: main has a; feat adds b, c (the commit to remove), d.
func linear(t *testing.T) (*scratch, string) {
	s := newScratch(t)
	s.commit("a", "a")
	s.in("checkout", "--quiet", "-b", "feat")
	s.commit("b", "b")
	c := s.commit("c", "c")
	s.commit("d", "d")
	s.push("main", "feat")
	return s, c
}

func TestRemoveCommit_DropAndRevert(t *testing.T) {
	zone.NewGlobal()
	s, c := linear(t)
	p := s.plan(c)
	require.Empty(t, p.dropWhy)
	require.Empty(t, p.revertWhy)

	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	require.Nil(t, m.openRemoveCommitDialog(p))
	view := ansi.Strip(m.dialog.View())
	require.Contains(t, view, "Remove a commit from #10?")
	require.Regexp(t, `Revert .*Drop .*Cancel`, strings.Join(strings.Fields(view), " "), "Revert first")

	// Enter presses Revert, the default.
	cmd := m.dialog.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, "a\nb\nc\nd", s.files("feat"), "not yet: the task runs it")
	msgs := runAll(t, cmd)
	require.Contains(t, msgs, tea.Msg(commitRemovedMsg{}))
	require.Equal(t, "a\nb\nd", s.files("feat"))
	require.Equal(t, p.head, s.git("-C", s.origin, "rev-parse", "feat~1"), "on top")

	// Dropping is planned again on the new tip.
	s.in("fetch", "--quiet", "origin")
	s.in("reset", "--quiet", "--hard", "origin/feat")
	p = s.plan(c)
	_ = runAll(t, m.pushRemoval(p, true))
	require.Equal(t, "a\nb\nd", s.files("feat"))
	require.Equal(t, "2", s.git("-C", s.origin, "rev-list", "--count", "feat", "^main", "--", "."),
		"b and d: c is gone, and its revert with it")
	require.NotContains(t, s.git("-C", s.origin, "log", "--format=%H", "feat"), c)
}

func TestRemoveCommit_DropRefusedWhenItTouchesMore(t *testing.T) {
	t.Run("a PR is based on the branch", func(t *testing.T) {
		s, c := linear(t)
		t.Setenv("GH_STACKED", "#11")
		p := s.plan(c)
		require.Empty(t, p.dropTip)
		require.Contains(t, strings.Join(p.dropWhy, ";"), "#11 based on feat")
		require.NotEmpty(t, p.revertTip, "revert stays")
	})
	t.Run("another branch has it", func(t *testing.T) {
		s, c := linear(t)
		s.in("branch", "copy", c)
		s.push("copy")
		p := s.plan(c)
		require.Contains(t, strings.Join(p.dropWhy, ";"), "other branches too: copy")
	})
	t.Run("a merge after it", func(t *testing.T) {
		s, c := linear(t)
		s.in("checkout", "--quiet", "main")
		s.commit("m", "m")
		s.in("checkout", "--quiet", "feat")
		s.in("merge", "--quiet", "--no-edit", "main")
		s.push("main", "feat")
		p := s.plan(c)
		require.Contains(t, strings.Join(p.dropWhy, ";"), "merges after it would be flattened")
	})
	t.Run("later commits need it", func(t *testing.T) {
		s := newScratch(t)
		s.commit("a", "a")
		s.in("checkout", "--quiet", "-b", "feat")
		c := s.commit("c", "c")
		s.commit("c", "c changed")
		s.push("main", "feat")
		p := s.plan(c)
		require.Contains(t, strings.Join(p.dropWhy, ";"), "conflicts")
		require.Contains(t, strings.Join(p.revertWhy, ";"), "conflicts")
		_, ok := m(t).openRemoveCommitDialog(p)().(constants.ErrMsg)
		require.True(t, ok, "nothing to offer")
	})
}

// As in deqalloc-artifact #10: the base merged in before the commit.
func TestRemoveCommit_DropAfterAMergeOfTheBase(t *testing.T) {
	s := newScratch(t)
	s.commit("a", "a")
	s.in("checkout", "--quiet", "-b", "feat")
	s.commit("b", "b")
	s.in("checkout", "--quiet", "main")
	s.commit("m", "m")
	s.in("checkout", "--quiet", "feat")
	s.in("merge", "--quiet", "--no-edit", "main")
	merge := s.in("rev-parse", "--short", "HEAD")
	c := s.commit("c", "c")
	s.commit("d", "d")
	s.push("main", "feat")
	p := s.plan(c)
	require.Empty(t, p.dropWhy)
	require.Equal(t, []string{"the merges before it stay: " + merge}, p.dropNotes)
	_ = runAll(t, m(t).pushRemoval(p, true))
	require.Equal(t, "a\nb\nd\nm", s.files("feat"))
	require.Equal(t, 2, len(strings.Fields(s.git("-C", s.origin, "rev-list", "--parents", "-n1", "feat~1")))-1,
		"the merge is still a merge")
}

func TestRemoveCommit_RefusedForcePushOffersRevert(t *testing.T) {
	s, c := linear(t)
	s.git("-C", s.origin, "config", "receive.denyNonFastForwards", "true")
	p := s.plan(c)
	m := m(t)
	var again removeCommitPlanMsg
	for _, msg := range runAll(t, m.pushRemoval(p, true)) {
		if r, ok := msg.(removeCommitPlanMsg); ok {
			again = r
		}
	}
	require.Contains(t, strings.Join(again.dropWhy, ";"), "GitHub refused the force-push")
	require.Equal(t, p.head, s.git("-C", s.origin, "rev-parse", "feat"), "branch untouched")
	require.Nil(t, m.openRemoveCommitDialog(again))
	view := strings.Join(strings.Fields(ansi.Strip(m.dialog.View())), " ")
	require.Contains(t, view, "Revert")
	require.NotRegexp(t, `│ *Drop *│`, ansi.Strip(m.dialog.View()))
}

func TestRemoveCommit_BranchMovedSincePlanned(t *testing.T) {
	s, c := linear(t)
	p := s.plan(c)
	s.commit("e", "e")
	s.push("feat")
	for _, msg := range runAll(t, m(t).pushRemoval(p, true)) {
		f, ok := msg.(constants.TaskFinishedMsg)
		if ok {
			require.Error(t, f.Err, "lease")
		}
	}
	require.Equal(t, "a\nb\nc\nd\ne", s.files("feat"))
}

func m(t *testing.T) *Model {
	zone.NewGlobal()
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth, m.ctx.ScreenHeight = 120, 40
	return &m
}

// runAll runs cmd and what its batches hold; it returns the other messages.
func runAll(t *testing.T, cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			out = append(out, runAll(t, c)...)
		}
	case nil:
	default:
		out = append(out, msg)
	}
	return out
}
