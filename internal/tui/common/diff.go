package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// ghdiffEnv tells ghdiff which PR it shows and where the repo is cloned
// (from repoPaths), so it can ask Claude about the diff and post to the PR.
// Other pagers ignore these.
func ghdiffEnv(env []string, repoName string, prNumber int, repoPaths map[string]string) []string {
	env = append(env, "GHDIFF_REPO="+repoName)
	if prNumber > 0 {
		env = append(env, fmt.Sprintf("GHDIFF_PR=%d", prNumber))
	}
	if path, ok := GetRepoLocalPath(repoName, repoPaths); ok {
		env = append(env, "GHDIFF_REPO_PATH="+path)
	}
	return env
}

// DiffPR opens a diff view for a PR using the gh CLI.
// The env parameter should be the result of Config.GetFullScreenDiffPagerEnv().
func DiffPR(prNumber int, repoName string, env []string, repoPaths map[string]string) tea.Cmd {
	c := exec.Command(
		"gh",
		"pr",
		"diff",
		fmt.Sprint(prNumber),
		"-R",
		repoName,
	)
	// GHDIFF_TITLE names the diff in ghdiff's title bar; other pagers ignore it.
	c.Env = append(ghdiffEnv(env, repoName, prNumber, repoPaths),
		fmt.Sprintf("GHDIFF_TITLE=%s#%d", repoName, prNumber))

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
// coloured here first. The env parameter is Config.GetFullScreenDiffPagerEnv();
// prNumber is the PR the commit belongs to.
func DiffCommit(repoName string, oid string, prNumber int, pager string, env []string,
	repoPaths map[string]string,
) tea.Cmd {
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
		c.Env = append(ghdiffEnv(env, repoName, prNumber, repoPaths),
			fmt.Sprintf("GHDIFF_TITLE=%s @ %s", repoName, oid[:min(7, len(oid))]),
			"GHDIFF_COMMIT="+oid)
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
