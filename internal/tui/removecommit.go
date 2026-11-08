package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// Removing the selected commit of a PR (U, on the Commits tab): by reverting
// it, a new commit on top, or by dropping it, rewriting the branch without it
// and force-pushing. The work is done in a clone of gh-dash's own
// ($XDG_CACHE_HOME/gh-dash/repos/OWNER/REPO), never in the user's checkouts.
//
// Both are tried first, in a scratch worktree of that clone, and the dialog
// says what was found: Drop is offered only when the rewrite touches nothing
// but this PR's branch and GitHub would take the force-push. Revert is the
// default. Pushing what was tried checks that the branch has not moved since.

// cloneRepo makes the clone of repo in dir; tests swap it.
var cloneRepo = func(repo, dir string) error {
	_, err := gh("repo", "clone", repo, dir, "--", "--filter=blob:none", "--no-checkout", "--quiet")
	return err
}

func cloneDir(repo string) (string, error) {
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "gh-dash", "repos", filepath.FromSlash(repo)), nil
}

func runGit(dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Stdout, c.Stderr = &stdout, &stderr
	if err := c.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String() + "\n" + stdout.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", args[0], msg)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// removeCommitPlanMsg is what trying both ways found.
type removeCommitPlanMsg struct {
	sid                 tasks.SectionIdentifier
	pr                  data.RowData
	repo, branch, clone string
	base                string // the PR's base branch
	sha, headline       string
	head                string // the branch's tip they were tried on
	revertTip, dropTip  string // the branch with the commit reverted or dropped
	revertWhy, dropWhy  []string
	dropNotes           []string // what to know before dropping
	err                 error
}

// commitRemovedMsg: the branch of a PR in section sid changed.
type commitRemovedMsg struct{ sid tasks.SectionIdentifier }

// planRemoveCommit tries, in the background, reverting and dropping commit
// sha of pr; the dialog opens on what it found.
func (m *Model) planRemoveCommit(sid tasks.SectionIdentifier, pr data.RowData, sha, headline string) tea.Cmd {
	row, ok := pr.(*prrow.Data)
	if !ok || row == nil || row.Primary == nil {
		return nil
	}
	p := row.Primary
	var why string
	switch {
	case p.State != "OPEN":
		why = "only an open PR's commits are removed"
	case p.IsCrossRepository:
		why = fmt.Sprintf("the branch %s is in a fork", p.HeadRefName)
	}
	if why != "" {
		return func() tea.Msg { return constants.ErrMsg{Err: errors.New(why)} }
	}
	plan := removeCommitPlanMsg{sid: sid, pr: pr, repo: pr.GetRepoNameWithOwner(),
		branch: p.HeadRefName, base: p.BaseRefName, sha: sha, headline: headline}
	taskId := fmt.Sprintf("remove_commit_plan_%s", sha)
	start := m.ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Trying to revert and to drop %.7s", sha),
		FinishedText: fmt.Sprintf("Tried removing %.7s", sha),
		State:        context.TaskStart,
	})
	return tea.Batch(start, func() tea.Msg {
		plan.try()
		return finishThen(taskId, sid, plan.err, plan)
	})
}

// try fills in the plan.
func (p *removeCommitPlanMsg) try() {
	p.clone, p.err = cloneDir(p.repo)
	if p.err != nil {
		return
	}
	if _, err := os.Stat(filepath.Join(p.clone, ".git")); err != nil {
		if p.err = os.MkdirAll(filepath.Dir(p.clone), 0o755); p.err != nil {
			return
		}
		if p.err = cloneRepo(p.repo, p.clone); p.err != nil {
			return
		}
	}
	// All branches: whether others hold the commit is part of the question.
	if _, p.err = runGit(p.clone, "fetch", "--quiet", "--prune", "origin",
		"+refs/heads/*:refs/remotes/origin/*"); p.err != nil {
		return
	}
	if p.head, p.err = runGit(p.clone, "rev-parse", "refs/remotes/origin/"+p.branch); p.err != nil {
		return
	}
	if _, err := runGit(p.clone, "merge-base", "--is-ancestor", p.sha, p.head); err != nil {
		p.err = fmt.Errorf("%.7s is not on %s any more", p.sha, p.branch)
		return
	}
	parents, _ := runGit(p.clone, "rev-list", "--parents", "-n1", p.sha)
	nParents := len(strings.Fields(parents)) - 1

	p.checkDrop(nParents)

	// Both are tried for real, in a worktree that goes away after.
	wt, err := os.MkdirTemp("", "gh-dash-remove-commit-")
	if err != nil {
		p.err = err
		return
	}
	defer func() {
		_, _ = runGit(p.clone, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}()
	_, _ = runGit(p.clone, "worktree", "prune") // left by a gh-dash that died here
	if _, p.err = runGit(p.clone, "worktree", "add", "--quiet", "--detach", wt, p.head); p.err != nil {
		return
	}
	revert := []string{"revert", "--no-edit"}
	if nParents > 1 {
		revert = append(revert, "-m", "1") // undo what the merge brought in
	}
	if _, err := runGit(wt, append(revert, p.sha)...); err != nil {
		_, _ = runGit(wt, "revert", "--abort")
		p.revertWhy = append(p.revertWhy, "reverting it conflicts with later commits")
	} else {
		p.revertTip, _ = runGit(wt, "rev-parse", "HEAD")
	}
	if len(p.dropWhy) > 0 {
		return
	}
	if _, p.err = runGit(wt, "checkout", "--quiet", "--detach", p.head); p.err != nil {
		return
	}
	if _, err := runGit(wt, "rebase", "--quiet", "--onto", p.sha+"^", p.sha); err != nil {
		_, _ = runGit(wt, "rebase", "--abort")
		p.dropWhy = append(p.dropWhy, "the later commits do not apply without it (conflicts)")
		return
	}
	p.dropTip, _ = runGit(wt, "rev-parse", "HEAD")
}

// checkDrop lists what makes dropping the commit wrong: other PRs or
// branches that would keep the old history, merges a rebase would flatten,
// and GitHub refusing force-pushes to the branch.
func (p *removeCommitPlanMsg) checkDrop(nParents int) {
	if nParents != 1 {
		p.dropWhy = append(p.dropWhy, "it is a merge or root commit")
	}
	if out, err := gh("pr", "list", "-R", p.repo, "--base", p.branch, "--state", "open",
		"--json", "number", "--jq", `map("#\(.number)")|join(", ")`); err != nil {
		p.dropWhy = append(p.dropWhy, "cannot tell which PRs are based on it: "+err.Error())
	} else if out = strings.TrimSpace(out); out != "" {
		p.dropWhy = append(p.dropWhy, fmt.Sprintf("%s based on %s would keep the old commits", out, p.branch))
	}
	if out, _ := runGit(p.clone, "for-each-ref", "--contains", p.sha, "--format=%(refname:lstrip=3)",
		"refs/remotes/origin"); out != "" {
		var others []string
		for _, b := range strings.Fields(out) {
			if b != p.branch && b != "HEAD" {
				others = append(others, b)
			}
		}
		if len(others) > 0 {
			p.dropWhy = append(p.dropWhy, "it is in other branches too: "+strings.Join(others, ", "))
		}
	}
	if out, _ := runGit(p.clone, "rev-list", "--merges", "--abbrev-commit", p.sha+".."+p.head); out != "" {
		p.dropWhy = append(p.dropWhy, "merges after it would be flattened: "+strings.Join(strings.Fields(out), ", "))
	}
	if out, _ := runGit(p.clone, "rev-list", "--merges", "--abbrev-commit",
		"refs/remotes/origin/"+p.base+".."+p.sha); out != "" {
		p.dropNotes = append(p.dropNotes, "the merges before it stay: "+strings.Join(strings.Fields(out), ", "))
	}
	p.checkForcePush()
}

// checkForcePush asks GitHub whether a rule or a branch protection forbids
// force-pushing to the branch. When it can't tell, the push itself will.
func (p *removeCommitPlanMsg) checkForcePush() {
	b := strings.ReplaceAll(p.branch, "/", "%2F")
	if out, err := gh("api", "repos/"+p.repo+"/rules/branches/"+b, "--jq", ".[].type"); err == nil &&
		strings.Contains(out, "non_fast_forward") {
		p.dropWhy = append(p.dropWhy, "a ruleset forbids force-pushes to "+p.branch)
		return
	}
	protected, err := gh("api", "repos/"+p.repo+"/branches/"+b, "--jq", ".protected")
	if err != nil || strings.TrimSpace(protected) != "true" {
		return
	}
	allowed, err := gh("api", "repos/"+p.repo+"/branches/"+b+"/protection", "--jq", ".allow_force_pushes.enabled")
	switch {
	case err != nil:
		p.dropNotes = append(p.dropNotes, p.branch+" is protected: GitHub may refuse the force-push")
	case strings.TrimSpace(allowed) != "true":
		p.dropWhy = append(p.dropWhy, "its protection forbids force-pushes to "+p.branch)
	}
}

// openRemoveCommitDialog shows what the plan found and asks how to remove
// the commit, Revert first.
func (m *Model) openRemoveCommitDialog(p removeCommitPlanMsg) tea.Cmd {
	if p.err != nil {
		return func() tea.Msg { return constants.ErrMsg{Err: fmt.Errorf("cannot remove %.7s: %w", p.sha, p.err)} }
	}
	text := []string{fmt.Sprintf("%.7s %s", p.sha, p.headline), ""}
	var buttons []string
	if p.revertTip != "" {
		buttons = append(buttons, "Revert")
		text = append(text, "Revert: a new commit that undoes it, pushed on top.")
	} else {
		text = append(text, "Revert: no, "+strings.Join(p.revertWhy, "; ")+".")
	}
	if p.dropTip != "" {
		buttons = append(buttons, "Drop")
		text = append(text, "Drop: rewrites "+p.branch+" without it and force-pushes.")
		for _, n := range p.dropNotes {
			text = append(text, "  "+n)
		}
	} else {
		text = append(text, "Drop: no,")
		for _, w := range p.dropWhy {
			text = append(text, "  "+w)
		}
	}
	if len(buttons) == 0 {
		return func() tea.Msg {
			return constants.ErrMsg{Err: fmt.Errorf("cannot remove %.7s: %s", p.sha,
				strings.Join(append(p.revertWhy, p.dropWhy...), "; "))}
		}
	}
	m.dialog.Open(dialog.Spec{
		Title:   fmt.Sprintf("Remove a commit from #%d?", p.pr.GetNumber()),
		Text:    text,
		Buttons: append(buttons, "Cancel"),
		Submit: func(_ map[string]string, button string) tea.Cmd {
			return m.pushRemoval(p, button == "Drop")
		},
	})
	return nil
}

// pushRemoval pushes the tried branch, if the branch is still where it was
// tried. A refused force-push brings the dialog back with only Revert.
func (m *Model) pushRemoval(p removeCommitPlanMsg, drop bool) tea.Cmd {
	how, tip := "Reverting", p.revertTip
	args := []string{"push", "--quiet", "origin"}
	if drop {
		how, tip = "Dropping", p.dropTip
		args = append(args, "--force-with-lease=refs/heads/"+p.branch+":"+p.head)
	}
	args = append(args, tip+":refs/heads/"+p.branch)
	taskId := fmt.Sprintf("remove_commit_%s", p.sha)
	start := m.ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("%s %.7s on %s", how, p.sha, p.branch),
		FinishedText: fmt.Sprintf("%s %.7s on %s: pushed", how, p.sha, p.branch),
		State:        context.TaskStart,
	})
	return tea.Batch(start, func() tea.Msg {
		_, err := runGit(p.clone, args...)
		var msg tea.Msg = commitRemovedMsg{sid: p.sid}
		// Refused, not because the branch moved: offer what is left.
		if err != nil && drop && strings.Contains(err.Error(), "rejected") &&
			!strings.Contains(err.Error(), "stale info") && !strings.Contains(err.Error(), "fetch first") {
			p.dropTip, p.dropNotes = "", nil
			p.dropWhy = []string{"GitHub refused the force-push: " + lastLine(err.Error())}
			msg = p
		} else if err != nil {
			msg = nil
		}
		return finishThen(taskId, p.sid, err, msg)
	})
}

// finishThen ends the task and hands next, if any, to the UI itself (a
// TaskFinishedMsg's own Msg goes to the section).
func finishThen(taskId string, sid tasks.SectionIdentifier, err error, next tea.Msg) tea.Msg {
	done := func() tea.Msg {
		return constants.TaskFinishedMsg{TaskId: taskId, SectionId: sid.Id, SectionType: sid.Type, Err: err}
	}
	if next == nil {
		return done()
	}
	return tea.BatchMsg{done, func() tea.Msg { return next }}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// refreshSection loads section sid's rows again, as the refresh key does.
func (m *Model) refreshSection(sid tasks.SectionIdentifier) tea.Cmd {
	s := m.getSectionAt(sid.Id)
	if s == nil || s.GetType() != sid.Type {
		return nil
	}
	data.ClearEnrichmentCache()
	s.ResetRows()
	m.syncSidebar()
	return tea.Batch(append([]tea.Cmd{s.SetIsLoading(true)}, s.FetchNextPageSectionRows()...)...)
}
