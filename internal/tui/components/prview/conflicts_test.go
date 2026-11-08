package prview

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
