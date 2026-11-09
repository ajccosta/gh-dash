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
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
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
	head  string   // the PR's
	tree  string   // the merge, conflict markers in its files
	files []string // the conflicting ones; none: git merges it cleanly
}

var (
	conflictsMu    sync.Mutex
	conflictsCache = map[string]conflicts{}
	fetchMu        sync.Mutex // one fetch into a checkout at a time
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
// GitHub says it conflicts and nobody has looked yet, and which of the open
// PRs (the dashboard's) it conflicts with.
func (m *Model) FindConflicts(open []*data.PullRequestData) tea.Cmd {
	if m == nil || m.pr == nil || m.pr.Data.Primary == nil || m.ctx == nil || m.ctx.Config == nil {
		return nil
	}
	m.openPRs = open
	return tea.Batch(m.findBaseConflicts(), m.findPeerConflicts(open))
}

func (m *Model) findBaseConflicts() tea.Cmd {
	if m.pr.Data.Primary.Mergeable != "CONFLICTING" {
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
	dir = expandHome(dir)
	baseRef := fmt.Sprintf("refs/gh-dash/%d/%s", number, base)
	headRef := fmt.Sprintf("refs/gh-dash/%d/pr-%d", number, number)
	defer func() {
		_, _, _ = gitCode(dir, "update-ref", "-d", headRef)
		_, _, _ = gitCode(dir, "update-ref", "-d", baseRef)
	}()
	if err := fetch(dir, repo, fmt.Sprintf("+refs/heads/%s:%s", base, baseRef),
		fmt.Sprintf("+refs/pull/%d/head:%s", number, headRef)); err != nil {
		return merge{dir: dir}, err
	}
	return mergeRefs(dir, baseRef, headRef)
}

// mergeRefs merges ref head into ref base in memory. The conflict markers
// name the sides as the refs, without refs/.
func mergeRefs(dir, base, head string) (merge, error) {
	res := merge{dir: dir}
	out, _, err := gitCode(dir, "rev-parse", base, head)
	if err != nil {
		return res, err
	}
	if oids := strings.Fields(out); len(oids) == 2 {
		res.base, res.head = oids[0], oids[1]
	}
	base, head = strings.TrimPrefix(base, "refs/"), strings.TrimPrefix(head, "refs/")
	out, code, err := gitCode(dir, "merge-tree", "--write-tree", "--name-only", "--no-messages", base, head)
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

// fetch fetches refspecs from repo into the checkout at dir.
func fetch(dir, repo string, refspecs ...string) error {
	fetchMu.Lock()
	defer fetchMu.Unlock()
	_, _, err := gitCode(dir, append([]string{"fetch", "-q", "--no-tags", "--no-write-fetch-head",
		conflictsRemote(repo)}, refspecs...)...)
	return err
}

// gitCode runs git in dir; code is 1 when merge-tree found conflicts.
func gitCode(dir string, args ...string) (out string, code int, err error) {
	out, err = runGit(dir, args...)
	var exit *exec.ExitError
	if args[0] == "merge-tree" && errors.As(err, &exit) {
		return out, 1, nil
	}
	return out, 0, err
}

// expandHome expands a leading ~/ of a repoPaths entry.
func expandHome(dir string) string {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return dir
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

// conflictEntry is one entry of the Conflicts tab: a merge's conflicting
// files to diff, all of them or one.
type conflictEntry struct {
	merge
	diff  []string
	title string // the pager's
}

// conflictEntries are the Conflicts tab's entries: for the base, then each
// open PR the shown one conflicts with, "All conflicting files" and then each
// file.
func (m *Model) conflictEntries() []conflictEntry {
	if m.pr == nil || m.pr.Data.Primary == nil {
		return nil
	}
	p := m.pr.Data.Primary
	var es []conflictEntry
	add := func(g merge, title string) {
		title = fmt.Sprintf("GHDIFF_TITLE=%s#%d: %s", p.Repository.NameWithOwner, p.Number, title)
		es = append(es, conflictEntry{g, g.files, title})
		for _, f := range g.files {
			es = append(es, conflictEntry{g, []string{f}, title})
		}
	}
	if c, ok := m.shownConflicts(); ok && !c.pending && c.err == nil && len(c.files) > 0 {
		add(c.merge, "conflicts with "+p.BaseRefName)
	}
	for _, q := range m.shownPeerConflicts().found {
		add(q.merge, fmt.Sprintf("conflicts with %s once #%d is merged", p.BaseRefName, q.number))
	}
	return es
}

func (m *Model) numConflictEntries() int {
	return len(m.conflictEntries())
}

// openConflict shows entry e in the diff pager: the base against the merge,
// for that file or all of them. ghdiff opens side by side, each conflict's
// current version beside its incoming one; it is not told the PR, as the
// lines of the merge are not the PR's to comment on.
func (m *Model) openConflict(e int) tea.Cmd {
	es := m.conflictEntries()
	if e >= len(es) {
		return nil
	}
	c, cfg := es[e], m.ctx.Config
	repo := m.pr.Data.Primary.Repository.NameWithOwner
	return func() tea.Msg {
		ctx := 3
		for _, f := range c.diff {
			ctx = max(ctx, longestConflict(c.dir, c.tree+":"+f))
		}
		out, err := runGit(c.dir, append([]string{"diff", "--no-color", fmt.Sprintf("-U%d", ctx),
			c.base, c.tree, "--"}, c.diff...)...)
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		return common.PageDiff([]byte(out), cfg.Pager.Diff,
			common.GhdiffEnv(cfg.GetFullScreenDiffPagerEnv(), repo, 0, cfg.RepoPaths), c.title, "GHDIFF_SPLIT=1")
	}
}

// longestConflict is how many lines the longest conflict in the blob has.
// The diff gets as many lines of context, so that each conflict is one hunk:
// its current side is unchanged lines, which would split a hunk otherwise.
func longestConflict(dir, blob string) int {
	text, err := runGit(dir, "cat-file", "-p", blob)
	if err != nil {
		return 0
	}
	longest, start := 0, -1
	for i, l := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(l, "<<<<<<< "):
			start = i
		case strings.HasPrefix(l, ">>>>>>> ") && start >= 0:
			longest, start = max(longest, i-start+1), -1
		}
	}
	return longest
}

// renderConflicts is the Overview's Conflicts section: the base's while
// GitHub says the PR conflicts, the open PRs' when it conflicts with some.
func (m *Model) renderConflicts() string {
	var parts []string
	e := 0
	if _, ok := m.shownConflicts(); ok {
		parts = append(parts, m.renderConflictList())
		e = m.numBaseConflictEntries()
	}
	if s := m.renderPeerConflicts(e, true); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

// renderConflictsTab is the Conflicts tab.
func (m *Model) renderConflictsTab() string {
	var parts []string
	if m.focused && m.tabPicker() == conflictPicker && m.numConflictEntries() > 0 {
		parts = append(parts, m.ctx.Styles.Common.FaintTextStyle.Render(
			"↑/↓ select · enter/d diff · ← tabs, list · esc list"))
	}
	if _, ok := m.shownConflicts(); ok {
		parts = append(parts, m.renderConflictList())
	} else {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.SuccessGlyph, " ",
			m.ctx.Styles.Common.FaintTextStyle.Render("No conflicts with "+m.pr.Data.Primary.BaseRefName)))
	}
	if s := m.renderPeerConflicts(m.numBaseConflictEntries(), false); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

// numBaseConflictEntries is how many of the entries are the base's.
func (m *Model) numBaseConflictEntries() int {
	if c, ok := m.shownConflicts(); ok && !c.pending && c.err == nil && len(c.files) > 0 {
		return len(c.files) + 1
	}
	return 0
}

// renderConflictList says what the PR conflicts in with its base, each file
// to open.
func (m *Model) renderConflictList() string {
	c, _ := m.shownConflicts()
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2)
	head := lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.FailureGlyph, " ",
		fmt.Sprintf("Conflicts with %s", m.pr.Data.Primary.BaseRefName))
	switch {
	case c.pending:
		return head + "\n" + faint.Render("Finding the files…")
	case c.err != nil:
		return head + "\n" + faint.Render(c.err.Error())
	case len(c.files) == 0:
		return head + "\n" + faint.Width(m.getIndentedContentWidth()).Render(fmt.Sprintf("GitHub can't merge it, but git can: it merges "+
			"from one common ancestor, git from all of them. Merging (m) merges %s into %s with git first.",
			m.pr.Data.Primary.BaseRefName, m.pr.Data.Primary.HeadRefName))
	}
	e := 0
	return strings.Join(append([]string{head}, m.conflictGroup(&e, c.files)...), "\n")
}

// conflictGroup is the entries of one merge's conflicting files, from *e on:
// all of them, then each.
func (m *Model) conflictGroup(e *int, files []string) []string {
	main := m.ctx.Styles.Common.MainTextStyle
	entry := func(text string, style lipgloss.Style) string {
		if m.selected(conflictPicker, *e) {
			text = main.Bold(true).Render(CursorMarker) + " " + main.Bold(true).Reverse(true).Render(text)
		} else {
			text = "  " + style.Render(text)
		}
		text = mark(conflictPicker, *e, text)
		*e++
		return text
	}
	lines := []string{entry(fmt.Sprintf("All conflicting files (%d), in the diff", len(files)),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText).Underline(true))}
	for _, f := range files {
		lines = append(lines, entry(f, main))
	}
	return lines
}

// BaseMerge is a PR GitHub can't merge but git can. GitHub merges from one
// common ancestor of the PR and its base; where they have several (each merged
// the other in at some point), git merges those first and may get through
// without a conflict. Merging the base into the PR's branch, as git does,
// leaves a single one, and GitHub can merge the PR then.
type BaseMerge struct {
	merge
	Repo, Base, Head string // the PR's base and head branches
	Number           int
}

// BaseMerge is set while GitHub says the shown PR conflicts and git merged it
// cleanly.
func (m *Model) BaseMerge() (BaseMerge, bool) {
	c, ok := m.shownConflicts()
	if !ok || c.pending || c.err != nil || len(c.files) > 0 || c.head == "" ||
		m.pr.Data.Primary.IsCrossRepository {
		return BaseMerge{}, false
	}
	p := m.pr.Data.Primary
	return BaseMerge{merge: c.merge, Repo: p.Repository.NameWithOwner,
		Base: p.BaseRefName, Head: p.HeadRefName, Number: p.Number}, true
}

// mergeablePolls is how often, mergeableEvery apart, GitHub is asked whether
// the PR can be merged now; tests shorten them.
var (
	mergeablePolls = 30
	mergeableEvery = 2 * time.Second
)

// Push commits git's merge of the base into the PR's branch and pushes it,
// never forced: if the branch moved since, the push fails. It returns once
// GitHub says the PR can be merged.
func (b BaseMerge) Push() error {
	commit, err := runGit(b.dir, "commit-tree", b.tree, "-p", b.head, "-p", b.base,
		"-m", fmt.Sprintf("Merge branch '%s' into %s", b.Base, b.Head))
	if err != nil {
		return err
	}
	if _, err := runGit(b.dir, "push", "-q", conflictsRemote(b.Repo),
		strings.TrimSpace(commit)+":refs/heads/"+b.Head); err != nil {
		return err
	}
	commit = strings.TrimSpace(commit)
	for range mergeablePolls {
		time.Sleep(mergeableEvery)
		out, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s/pulls/%d", b.Repo, b.Number),
			"--jq", `"\(.head.sha) \(.mergeable)"`).Output()
		if err != nil {
			return fmt.Errorf("gh api: %w", err)
		}
		switch strings.TrimSpace(string(out)) { // until GitHub has seen the push
		case commit + " true":
			return nil
		case commit + " false":
			return fmt.Errorf("pushed the merge of %s into %s, but GitHub still can't merge #%d", b.Base, b.Head, b.Number)
		}
	}
	return fmt.Errorf("pushed the merge of %s into %s; GitHub has not said yet whether #%d can be merged", b.Base, b.Head, b.Number)
}
