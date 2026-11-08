package prview

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// The files a conflicting PR conflicts in, on the Overview and the Conflicts
// tab: GitHub only says that it conflicts. Git works them out in the repo's
// local checkout (repoPaths) without touching its work tree: the PR's head and
// base are fetched into refs of their own, merged with `git merge-tree`, and
// the refs removed again. Opening a file diffs the base against the merge,
// conflict markers and all.

// conflicts is what git found, by conflictKey; a PR is looked at again once
// it changes (its UpdatedAt).
type conflicts struct {
	pending bool
	merge
	err error
}

// merge is the PR merged into its base by git, in the checkout at dir.
type merge struct {
	dir   string
	base  string   // the base's commit
	tree  string   // the merge, conflict markers in its files
	files []string // the conflicting ones; none: git merges it cleanly
}

var (
	conflictsMu    sync.Mutex
	conflictsCache = map[string]conflicts{}
)

// conflictsRemote is where the PR is fetched from.
var conflictsRemote = func(repo string) string { return "https://github.com/" + repo + ".git" }

// ConflictsMsg says git is done with a PR; the preview shows the result.
type ConflictsMsg struct{}

func (m *Model) conflictKey() string {
	p := m.pr.Data.Primary
	return fmt.Sprintf("%s#%d@%d", p.Repository.NameWithOwner, p.Number, p.UpdatedAt.UnixNano())
}

// FindConflicts has git work out the files the shown PR conflicts in, if
// GitHub says it conflicts and nobody has looked yet.
func (m *Model) FindConflicts() tea.Cmd {
	if m == nil || m.pr == nil || m.pr.Data.Primary == nil ||
		m.pr.Data.Primary.Mergeable != "CONFLICTING" || m.ctx == nil || m.ctx.Config == nil {
		return nil
	}
	p := m.pr.Data.Primary
	key := m.conflictKey()
	conflictsMu.Lock()
	defer conflictsMu.Unlock()
	if _, ok := conflictsCache[key]; ok {
		return nil
	}
	repo := p.Repository.NameWithOwner
	dir, ok := common.GetRepoLocalPath(repo, m.ctx.Config.RepoPaths)
	if !ok {
		conflictsCache[key] = conflicts{err: fmt.Errorf("add %s to repoPaths to see which files", repo)}
		return nil
	}
	conflictsCache[key] = conflicts{pending: true}
	number, base := p.Number, p.BaseRefName
	return func() tea.Msg {
		res, err := conflictingFiles(dir, repo, number, base)
		conflictsMu.Lock()
		conflictsCache[key] = conflicts{merge: res, err: err}
		conflictsMu.Unlock()
		return ConflictsMsg{}
	}
}

// conflictingFiles merges PR number of repo into base in the checkout at dir,
// in memory. The conflict markers name the two sides as their refs:
// gh-dash/<number>/<base> and gh-dash/<number>/pr-<number>.
func conflictingFiles(dir, repo string, number int, base string) (merge, error) {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, rest)
		}
	}
	res := merge{dir: dir}
	baseRef := fmt.Sprintf("gh-dash/%d/%s", number, base)
	headRef := fmt.Sprintf("gh-dash/%d/pr-%d", number, number)
	git := func(args ...string) (string, int, error) {
		out, err := runGit(dir, args...)
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out, 1, nil // merge-tree: conflicts
		}
		return out, 0, err
	}
	defer func() {
		_, _, _ = git("update-ref", "-d", "refs/"+headRef)
		_, _, _ = git("update-ref", "-d", "refs/"+baseRef)
	}()
	if _, _, err := git("fetch", "-q", "--no-tags", "--no-write-fetch-head",
		conflictsRemote(repo),
		fmt.Sprintf("+refs/heads/%s:refs/%s", base, baseRef),
		fmt.Sprintf("+refs/pull/%d/head:refs/%s", number, headRef)); err != nil {
		return res, err
	}
	out, _, err := git("rev-parse", "refs/"+baseRef)
	if err != nil {
		return res, err
	}
	res.base = strings.TrimSpace(out)
	out, code, err := git("merge-tree", "--write-tree", "--name-only", "--no-messages", baseRef, headRef)
	if err != nil {
		return res, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	res.tree = lines[0]
	if code == 1 {
		res.files = lines[1:]
	}
	return res, nil
}

// runGit runs git in dir; an error carries what git said.
func runGit(dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && args[0] == "merge-tree" {
		return stdout.String(), err // conflicts
	}
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = errors.New(msg)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return stdout.String(), nil
}

// shownConflicts is what git found for the shown PR, while GitHub says it
// conflicts; ok false when it does not.
func (m *Model) shownConflicts() (c conflicts, ok bool) {
	p := m.pr.Data.Primary
	if p == nil || p.Mergeable != "CONFLICTING" || p.State != "OPEN" {
		return c, false
	}
	conflictsMu.Lock()
	defer conflictsMu.Unlock()
	c, found := conflictsCache[m.conflictKey()]
	if !found {
		c.pending = true
	}
	return c, true
}

// The Conflicts tab's entries: "All conflicting files" (entry 0), then each
// file (entry i+1 is file i).
func (m *Model) numConflictEntries() int {
	if c, ok := m.shownConflicts(); ok && !c.pending && c.err == nil && len(c.files) > 0 {
		return len(c.files) + 1
	}
	return 0
}

// openConflict shows entry e in the diff pager: the base against the merge,
// for that file or all of them. ghdiff is not told the PR, as the lines of the
// merge are not the PR's to comment on.
func (m *Model) openConflict(e int) tea.Cmd {
	c, _ := m.shownConflicts()
	if e > len(c.files) {
		return nil
	}
	files := c.files
	if e > 0 {
		files = files[e-1 : e]
	}
	p, cfg := m.pr.Data.Primary, m.ctx.Config
	repo := p.Repository.NameWithOwner
	title := fmt.Sprintf("GHDIFF_TITLE=%s#%d: conflicts with %s", repo, p.Number, p.BaseRefName)
	return func() tea.Msg {
		out, err := runGit(c.dir, append([]string{"diff", "--no-color", c.base, c.tree, "--"}, files...)...)
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		return common.PageDiff([]byte(out), cfg.Pager.Diff,
			common.GhdiffEnv(cfg.GetFullScreenDiffPagerEnv(), repo, 0, cfg.RepoPaths), title)
	}
}

// renderConflicts is the Overview's Conflicts section, while GitHub says the
// PR conflicts.
func (m *Model) renderConflicts() string {
	if _, ok := m.shownConflicts(); !ok {
		return ""
	}
	return m.renderConflictList()
}

// renderConflictsTab is the Conflicts tab.
func (m *Model) renderConflictsTab() string {
	if _, ok := m.shownConflicts(); !ok {
		return lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.SuccessGlyph, " ",
			m.ctx.Styles.Common.FaintTextStyle.Render("No conflicts with "+m.pr.Data.Primary.BaseRefName))
	}
	return m.renderConflictList()
}

// renderConflictList says what the PR conflicts in, each file to open.
func (m *Model) renderConflictList() string {
	c, _ := m.shownConflicts()
	main := m.ctx.Styles.Common.MainTextStyle
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2)
	head := lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.FailureGlyph, " ",
		fmt.Sprintf("Conflicts with %s", m.pr.Data.Primary.BaseRefName))
	switch {
	case c.pending:
		return head + "\n" + faint.Render("Finding the files…")
	case c.err != nil:
		return head + "\n" + faint.Render(c.err.Error())
	case len(c.files) == 0:
		return head + "\n" + faint.Render("Git merges it cleanly: GitHub may not have checked again yet")
	}
	lines := []string{head}
	if m.focused && m.tabPicker() == conflictPicker {
		lines = append(lines, m.ctx.Styles.Common.FaintTextStyle.Render(
			"↑/↓ select · enter/d diff · ← tabs, list · esc list"))
	}
	entry := func(e int, text string, style lipgloss.Style) string {
		if m.selected(conflictPicker, e) {
			text = main.Bold(true).Render(CursorMarker) + " " + main.Bold(true).Reverse(true).Render(text)
		} else {
			text = "  " + style.Render(text)
		}
		return mark(conflictPicker, e, text)
	}
	lines = append(lines, entry(0, fmt.Sprintf("All conflicting files (%d), in the diff", len(c.files)),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText).Underline(true)))
	for i, f := range c.files {
		lines = append(lines, entry(i+1, f, main))
	}
	return strings.Join(lines, "\n")
}
