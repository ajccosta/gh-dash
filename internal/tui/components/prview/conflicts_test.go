package prview

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// gitRepo makes a repo whose main and PR #1 (refs/pull/1/head) both change
// a.txt, and b.txt only on main, and a clone of it.
func gitRepo(t *testing.T) (remote, clone string) {
	t.Helper()
	run := func(dir string, args ...string) {
		c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := c.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(dir, name, text string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}
	remote, clone = t.TempDir(), t.TempDir()
	run(remote, "init", "-q", "-b", "main")
	write(remote, "a.txt", "one\n")
	write(remote, "b.txt", "one\n")
	run(remote, "add", ".")
	run(remote, "commit", "-qm", "start")
	run(remote, "checkout", "-qb", "pr")
	write(remote, "a.txt", "pr\n")
	run(remote, "commit", "-qam", "pr")
	run(remote, "update-ref", "refs/pull/1/head", "pr")
	run(remote, "checkout", "-q", "main")
	write(remote, "a.txt", "main\n")
	write(remote, "b.txt", "main\n")
	run(remote, "commit", "-qam", "main")
	run(clone, "init", "-q")
	return remote, clone
}

func TestConflictingFiles(t *testing.T) {
	remote, clone := gitRepo(t)
	conflictsRemote = func(string) string { return remote }
	t.Cleanup(func() { conflictsRemote = func(repo string) string { return "https://github.com/" + repo + ".git" } })

	res, err := conflictingFiles(clone, "o/r", 1, "main")
	require.NoError(t, err)
	require.Equal(t, []string{"a.txt"}, res.files)

	diff, err := runGit(clone, "diff", "--no-color", res.base, res.tree, "--", "a.txt")
	require.NoError(t, err)
	require.Contains(t, diff, "+<<<<<<< gh-dash/1/main\n main\n+=======\n+pr\n+>>>>>>> gh-dash/1/pr-1",
		"the base against the merge, its sides named")
	require.Equal(t, 5, longestConflict(clone, res.tree+":a.txt"))

	out, err := exec.Command("git", "-C", clone, "for-each-ref", "refs/gh-dash").Output()
	require.NoError(t, err)
	require.Empty(t, string(out), "its refs are removed again")

	_, err = conflictingFiles(clone, "o/r", 1, "gone")
	require.ErrorContains(t, err, "git fetch")
}

func TestBaseMergePush(t *testing.T) {
	remote, clone := gitRepo(t)
	conflictsRemote = func(string) string { return remote }
	t.Cleanup(func() { conflictsRemote = func(repo string) string { return "https://github.com/" + repo + ".git" } })
	for _, k := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+k+"_NAME", "t")
		t.Setenv("GIT_"+k+"_EMAIL", "t@t")
	}
	// The PR's branch changes a.txt as main does: undo main's change to it, so
	// git merges cleanly.
	git := func(args ...string) string {
		out, err := runGit(remote, append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		require.NoError(t, err)
		return strings.TrimSpace(out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(remote, "a.txt"), []byte("pr\n"), 0o644))
	git("commit", "-qam", "main takes pr's a.txt")

	res, err := conflictingFiles(clone, "o/r", 1, "main")
	require.NoError(t, err)
	require.Empty(t, res.files)

	// gh says the PR can be merged once its head is what was pushed.
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"$(git -C '" + remote + "' rev-parse pr) true\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	mergeableEvery = 0
	t.Cleanup(func() { mergeableEvery = 2 * time.Second })

	b := BaseMerge{merge: res, Repo: "o/r", Base: "main", Head: "pr", Number: 1}
	require.NoError(t, b.Push())
	require.Equal(t, res.head+" "+res.base, git("log", "-1", "--format=%P", "pr"), "the PR, then main")
	require.Equal(t, res.tree, git("rev-parse", "pr^{tree}"))
	require.Equal(t, "Merge branch 'main' into pr", git("log", "-1", "--format=%s", "pr"))

	// The branch moved since: the push is refused, never forced.
	b.head = res.base
	require.ErrorContains(t, b.Push(), "git push")
}
