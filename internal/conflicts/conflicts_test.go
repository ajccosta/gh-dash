package conflicts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWithPeersEachOther: from main, #1 and #2 change a.txt each their way,
// #3 changes b.txt, #4 changes a.txt as #1 does, #5 conflicts with main.
func TestWithPeersEachOther(t *testing.T) {
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
	pr := func(n, file, text string) {
		run("checkout", "-qb", "pr"+n, "main")
		write(file, text)
		run("commit", "-qam", "pr "+n)
		run("update-ref", "refs/pull/"+n+"/head", "pr"+n)
	}
	pr("1", "a.txt", "one's\n")
	pr("2", "a.txt", "two's\n")
	pr("3", "b.txt", "three's\n")
	pr("4", "a.txt", "one's\n")
	pr("5", "b.txt", "five's\n")
	run("checkout", "-q", "main")
	write("b.txt", "main's\n")
	run("commit", "-qam", "main")
	run("init", "-q", clone)
	Remote = func(string) string { return remote }
	t.Cleanup(func() { Remote = func(repo string) string { return "https://github.com/" + repo + ".git" } })

	all := []int{1, 2, 3, 4, 5}
	found, err := WithPeers(clone, "o/r", "main", "list/1", all, all)
	require.NoError(t, err)
	numbers := map[int][]int{}
	for n, ps := range found {
		for _, p := range ps {
			numbers[n] = append(numbers[n], p.Number)
		}
	}
	require.Equal(t, map[int][]int{1: {2}, 2: {1, 4}, 4: {2}}, numbers,
		"each pair both ways; #3 another file, #5 not mergeable into main as is")

	out, err := Git(clone, "for-each-ref", "refs/gh-dash")
	require.NoError(t, err)
	require.Empty(t, out, "its refs are removed again")
}
