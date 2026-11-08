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
)

// The files a conflicting PR conflicts in, under "This branch has conflicts":
// GitHub only says that it conflicts. Git works them out in the repo's local
// checkout (repoPaths) without touching its work tree: the PR's head and base
// are fetched into refs of their own, merged with `git merge-tree`, and the
// refs removed again.

// conflicts is what git found, by conflictKey; a PR is looked at again once
// it changes (its UpdatedAt).
type conflicts struct {
	pending bool
	files   []string // none: git merges it cleanly
	err     error
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
		files, err := conflictingFiles(dir, repo, number, base)
		conflictsMu.Lock()
		conflictsCache[key] = conflicts{files: files, err: err}
		conflictsMu.Unlock()
		return ConflictsMsg{}
	}
}

// conflictingFiles merges PR number of repo into base in the checkout at dir,
// in memory, and returns the files that conflict.
func conflictingFiles(dir, repo string, number int, base string) ([]string, error) {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, rest)
		}
	}
	ns := fmt.Sprintf("refs/gh-dash/conflicts/%d", number)
	git := func(args ...string) (string, int, error) {
		var stdout, stderr bytes.Buffer
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Stdout, c.Stderr = &stdout, &stderr
		err := c.Run()
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 && args[0] == "merge-tree" {
			return stdout.String(), 1, nil // conflicts
		}
		if err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				err = errors.New(msg)
			}
			return "", 0, fmt.Errorf("git %s: %w", args[0], err)
		}
		return stdout.String(), 0, nil
	}
	defer func() {
		_, _, _ = git("update-ref", "-d", ns+"/head")
		_, _, _ = git("update-ref", "-d", ns+"/base")
	}()
	if _, _, err := git("fetch", "-q", "--no-tags", "--no-write-fetch-head",
		conflictsRemote(repo),
		fmt.Sprintf("+refs/heads/%s:%s/base", base, ns),
		fmt.Sprintf("+refs/pull/%d/head:%s/head", number, ns)); err != nil {
		return nil, err
	}
	out, code, err := git("merge-tree", "--write-tree", "--name-only", "--no-messages",
		ns+"/base", ns+"/head")
	if err != nil || code == 0 {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lines[1:], nil // after the tree
}

// renderConflicts is the Conflicts section: the files, while GitHub says the
// PR conflicts.
func (m *Model) renderConflicts() string {
	p := m.pr.Data.Primary
	if p == nil || p.Mergeable != "CONFLICTING" || p.State != "OPEN" {
		return ""
	}
	conflictsMu.Lock()
	c, ok := conflictsCache[m.conflictKey()]
	conflictsMu.Unlock()
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2)
	head := lipgloss.JoinHorizontal(lipgloss.Top, m.ctx.Styles.Common.FailureGlyph, " ",
		fmt.Sprintf("Conflicts with %s", p.BaseRefName))
	switch {
	case !ok || c.pending:
		return head + "\n" + faint.Render("Finding the files…")
	case c.err != nil:
		return head + "\n" + faint.Render(c.err.Error())
	case len(c.files) == 0:
		return head + "\n" + faint.Render("Git merges it cleanly: GitHub may not have checked again yet")
	}
	lines := []string{head}
	for _, f := range c.files {
		lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(f))
	}
	return strings.Join(lines, "\n")
}
