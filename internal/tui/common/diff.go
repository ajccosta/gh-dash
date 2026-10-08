package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// DiffPR opens a diff view for a PR using the gh CLI.
// The env parameter should be the result of Config.GetFullScreenDiffPagerEnv().
func DiffPR(prNumber int, repoName string, env []string) tea.Cmd {
	c := exec.Command(
		"gh",
		"pr",
		"diff",
		fmt.Sprint(prNumber),
		"-R",
		repoName,
	)
	// GHDIFF_TITLE names the diff in ghdiff's title bar; other pagers ignore it.
	c.Env = append(env, fmt.Sprintf("GHDIFF_TITLE=%s#%d", repoName, prNumber))

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		return nil
	})
}

// DiffCommit shows a single commit's diff in the configured pager. The diff is
// fetched with `gh api`, so no local clone is needed. Pagers that colour diffs
// themselves (delta, diff-so-fancy, ...) get it raw; for less, lines are
// coloured here first. The env parameter is Config.GetFullScreenDiffPagerEnv().
func DiffCommit(repoName string, oid string, pager string, env []string) tea.Cmd {
	// Fetch in the background; the returned message then hands the terminal
	// to the pager, the same way tea.ExecProcess does.
	return func() tea.Msg {
		out, err := exec.Command(
			"gh", "api",
			"-H", "Accept: application/vnd.github.diff",
			fmt.Sprintf("repos/%s/commits/%s", repoName, oid),
		).Output()
		if err != nil {
			return constants.ErrMsg{Err: fmt.Errorf("fetching commit %s: %w", oid, err)}
		}

		if pager == "" {
			pager = "less"
		}
		if strings.HasPrefix(pager, "less") {
			out = colorDiff(out)
		}
		if pager == "delta" {
			pager = "delta --paging always"
		}

		f, err := os.CreateTemp("", "gh-dash-commit-*.diff")
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		_, werr := f.Write(out)
		f.Close()
		if werr != nil {
			os.Remove(f.Name())
			return constants.ErrMsg{Err: werr}
		}

		c := exec.Command("sh", "-c", pager+` < "$1"`, "sh", f.Name())
		c.Env = append(env, fmt.Sprintf("GHDIFF_TITLE=%s @ %s", repoName, oid[:min(7, len(oid))]))
		return tea.ExecProcess(c, func(err error) tea.Msg {
			os.Remove(f.Name())
			if err != nil {
				return constants.ErrMsg{Err: err}
			}
			return nil
		})()
	}
}

// colorDiff adds the usual ANSI colours to a unified diff.
func colorDiff(diff []byte) []byte {
	const (
		reset = "\x1b[0m"
		bold  = "\x1b[1m"
		red   = "\x1b[31m"
		green = "\x1b[32m"
		cyan  = "\x1b[36m"
	)
	lines := strings.Split(string(diff), "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git"), strings.HasPrefix(l, "index "),
			strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "+++ "):
			lines[i] = bold + l + reset
		case strings.HasPrefix(l, "@@"):
			lines[i] = cyan + l + reset
		case strings.HasPrefix(l, "+"):
			lines[i] = green + l + reset
		case strings.HasPrefix(l, "-"):
			lines[i] = red + l + reset
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
