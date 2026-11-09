package prview

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPeerConflictingFiles: from main, #1 and #2 change a.txt each their way,
// #3 changes b.txt, #4 changes a.txt as #1 does and c.txt.
func TestPeerConflictingFiles(t *testing.T) {
	remote, clone := t.TempDir(), t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", remote, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := c.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(name, text string) {
		require.NoError(t, os.WriteFile(filepath.Join(remote, name), []byte(text), 0o644))
	}
	run("init", "-q", "-b", "main")
	write("a.txt", "one\n")
	write("b.txt", "one\n")
	run("add", ".")
	run("commit", "-qm", "start")
	pr := func(n string, files map[string]string) {
		run("checkout", "-qb", "pr"+n, "main")
		for f, text := range files {
			write(f, text)
		}
		run("add", ".")
		run("commit", "-qm", "pr "+n)
		run("update-ref", "refs/pull/"+n+"/head", "pr"+n)
	}
	pr("1", map[string]string{"a.txt": "one's\n"})
	pr("2", map[string]string{"a.txt": "two's\n"})
	pr("3", map[string]string{"b.txt": "three's\n"})
	pr("4", map[string]string{"a.txt": "one's\n", "c.txt": "four's\n"})
	run("init", "-q", clone)
	conflictsRemote = func(string) string { return remote }
	t.Cleanup(func() { conflictsRemote = func(repo string) string { return "https://github.com/" + repo + ".git" } })

	found, err := peerConflictingFiles(clone, "o/r", 1, "main", []int{2, 3, 4})
	require.NoError(t, err)
	require.Len(t, found, 1, "#3 changes another file, #4 a.txt as #1 does")
	require.Equal(t, 2, found[0].number)
	require.Equal(t, []string{"a.txt"}, found[0].files)

	diff, err := runGit(clone, "diff", "--no-color", found[0].base, found[0].tree, "--", "a.txt")
	require.NoError(t, err)
	require.Contains(t, diff, "+<<<<<<< gh-dash/peers/1/main+pr-2\n two's\n+=======\n+one's\n+>>>>>>> gh-dash/peers/1/pr-1",
		"main with #2 merged against the merge, its sides named")

	out, err := runGit(clone, "for-each-ref", "refs/gh-dash")
	require.NoError(t, err)
	require.Empty(t, out, "its refs are removed again")
}
