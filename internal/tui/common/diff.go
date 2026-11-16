package common

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/conflicts"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// GhdiffEnv tells ghdiff which PR it shows (none: prNumber 0) and where the
// repo is cloned (from repoPaths), so it can ask Claude about the diff and
// post to the PR. Other pagers ignore these.
func GhdiffEnv(env []string, repoName string, prNumber int, repoPaths map[string]string) []string {
	env = append(env, "GHDIFF_REPO="+repoName)
	if prNumber > 0 {
		env = append(env, fmt.Sprintf("GHDIFF_PR=%d", prNumber))
	}
	if path, ok := GetRepoLocalPath(repoName, repoPaths); ok {
		env = append(env, "GHDIFF_REPO_PATH="+path)
	}
	return env
}

// DiffPR opens a diff view for a PR. The diff is fetched with the gh CLI, or,
// when GitHub won't send it (over 20,000 lines), made by git in the repo's
// local checkout (repoPaths), then shown in the pager.
// The env parameter should be the result of Config.GetFullScreenDiffPagerEnv();
// extraEnv is added to it (such as GHDIFF_GOTO, to open at some lines).
func DiffPR(prNumber int, repoName string, env []string, repoPaths map[string]string,
	extraEnv ...string,
) tea.Cmd {
	return func() tea.Msg {
		out, err := prDiff(prNumber, repoName, repoPaths)
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		// GHDIFF_TITLE names the diff in ghdiff's title bar; other pagers ignore it.
		return PageDiff(out, pagerOf(env), GhdiffEnv(env, repoName, prNumber, repoPaths),
			append([]string{fmt.Sprintf("GHDIFF_TITLE=%s#%d", repoName, prNumber)}, extraEnv...)...)
	}
}

// prDiff is the PR's diff from GitHub, or from git in the local checkout when
// GitHub says it is too large; an error says what gh or git said.
func prDiff(n int, repo string, repoPaths map[string]string) ([]byte, error) {
	out, msg, err := gh("pr", "diff", fmt.Sprint(n), "-R", repo, "--color", "never")
	if err == nil {
		return out, nil
	}
	if !strings.Contains(msg, "too_large") {
		return nil, fmt.Errorf("fetching #%d's diff: %s", n, msg)
	}
	dir, ok := GetRepoLocalPath(repo, repoPaths)
	if !ok {
		return nil, fmt.Errorf("#%d's diff is too large for GitHub to send; add %s to repoPaths to have git make it", n, repo)
	}
	base, msg, err := gh("pr", "view", fmt.Sprint(n), "-R", repo, "--json", "baseRefName", "-q", ".baseRefName")
	if err != nil {
		return nil, fmt.Errorf("finding #%d's base: %s", n, msg)
	}
	out, err = conflicts.PRDiff(dir, repo, strings.TrimSpace(string(base)), n)
	if err != nil {
		return nil, fmt.Errorf("making #%d's diff with git in %s: %w", n, dir, err)
	}
	return out, nil
}

// gh runs the gh CLI: its output, and what it said on stderr (or the error)
// when it fails.
func gh(args ...string) ([]byte, string, error) {
	var stderr bytes.Buffer
	c := exec.Command("gh", args...)
	c.Stderr = &stderr
	out, err := c.Output()
	msg := strings.TrimSpace(stderr.String())
	if err != nil && msg == "" {
		msg = err.Error()
	}
	return out, msg, err
}

// pagerOf is the diff pager in env (GH_PAGER), "" for the default.
func pagerOf(env []string) string {
	p := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GH_PAGER="); ok {
			p = v
		}
	}
	return p
}

// DiffCommit shows a single commit's diff in the configured pager. The diff is
// fetched with `gh api`, so no local clone is needed. The env parameter is
// Config.GetFullScreenDiffPagerEnv(); prNumber is the PR the commit belongs to.
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

		return PageDiff(out, pager, GhdiffEnv(env, repoName, prNumber, repoPaths),
			fmt.Sprintf("GHDIFF_TITLE=%s @ %s", repoName, oid[:min(7, len(oid))]),
			"GHDIFF_COMMIT="+oid)
	}
}

// PageDiff shows diff in pager (less by default), with env and extraEnv; it
// blocks, so it runs in a background command, whose message it returns.
// Pagers that colour diffs themselves (delta, diff-so-fancy, ...) get it raw;
// for less, lines are coloured here first.
func PageDiff(diff []byte, pager string, env []string, extraEnv ...string) tea.Msg {
	if pager == "" {
		pager = "less"
	}
	if strings.HasPrefix(pager, "less") {
		diff = colorDiff(diff)
	}
	if pager == "delta" {
		pager = "delta --paging always"
	}

	f, err := os.CreateTemp("", "gh-dash-*.diff")
	if err != nil {
		return constants.ErrMsg{Err: err}
	}
	_, werr := f.Write(diff)
	f.Close()
	if werr != nil {
		os.Remove(f.Name())
		return constants.ErrMsg{Err: werr}
	}

	c := exec.Command("sh", "-c", pager+` < "$1"`, "sh", f.Name())
	c.Env = append(env, extraEnv...)
	return ExecCovered(c, "Opening the diff…", func(err error) tea.Msg {
		os.Remove(f.Name())
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		return nil
	})()
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
