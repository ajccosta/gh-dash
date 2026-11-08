package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// Deleting the head branch of a merged PR (B), or of one merged with "Delete
// branch" in the merge dialog. GitHub is asked first which open PRs are based
// on that branch; the dialog lists them, and they are moved onto the merged
// PR's base before the branch goes, so none is left on a branch that no
// longer exists (GitHub would close them). The default branch is never
// deleted.

// maxStacked is how many PRs based on one branch are looked up; with more the
// branch is left alone, as some could not be moved first.
const maxStacked = 200

type stackedPR struct {
	Number int
	Title  string
}

// deleteBranchPlanMsg is what GitHub said about the branch of pr.
type deleteBranchPlanMsg struct {
	sid           tasks.SectionIdentifier
	pr            data.RowData
	branch, base  string
	defaultBranch string
	stacked       []stackedPR
	merge         string // the method to merge pr with first; the merge dialog asked already
	err           error
}

// planDeleteBranch asks GitHub, in the background, what deleting the head
// branch of pr involves; the dialog opens on the answer. With merge set, pr is
// merged first, with that method, and no dialog asks again.
func (m *Model) planDeleteBranch(sid tasks.SectionIdentifier, pr data.RowData, merge string) tea.Cmd {
	row, ok := pr.(*prrow.Data)
	if !ok || row == nil || row.Primary == nil {
		return nil
	}
	p := row.Primary
	var why string
	switch {
	case merge == "" && p.State != "MERGED":
		why = "only a merged PR's branch is deleted"
	case p.HeadRef.Name == "":
		why = fmt.Sprintf("the branch %s is deleted already", p.HeadRefName)
	case p.IsCrossRepository:
		why = fmt.Sprintf("the branch %s is in a fork", p.HeadRefName)
	}
	if why != "" {
		fail := func() tea.Msg { return constants.ErrMsg{Err: errors.New(why)} }
		if merge != "" {
			return tea.Batch(tasks.MergePR(m.ctx, sid, pr, merge, nil), fail)
		}
		return fail
	}
	repo, branch, base := pr.GetRepoNameWithOwner(), p.HeadRef.Name, p.BaseRefName
	return func() tea.Msg {
		plan := deleteBranchPlanMsg{sid: sid, pr: pr, branch: branch, base: base, merge: merge}
		var out string
		out, plan.err = gh("pr", "list", "-R", repo, "--base", branch, "--state", "open",
			"--json", "number,title", "--limit", fmt.Sprint(maxStacked))
		if plan.err == nil {
			plan.err = json.Unmarshal([]byte(out), &plan.stacked)
		}
		if plan.err == nil {
			out, plan.err = gh("repo", "view", repo, "--json", "defaultBranchRef",
				"--jq", ".defaultBranchRef.name")
			plan.defaultBranch = strings.TrimSpace(out)
		}
		return plan
	}
}

func gh(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("gh", args...)
	c.Stdout, c.Stderr = &stdout, &stderr
	if err := c.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return "", err
	}
	return stdout.String(), nil
}

// openDeleteBranchDialog asks to delete the branch the plan is about, listing
// the PRs it moves first, or says why it can't be.
func (m *Model) openDeleteBranchDialog(plan deleteBranchPlanMsg) tea.Cmd {
	fail := func(err error) tea.Cmd {
		failed := func() tea.Msg { return constants.ErrMsg{Err: err} }
		if plan.merge != "" { // merge still, keeping the branch
			return tea.Batch(tasks.MergePR(m.ctx, plan.sid, plan.pr, plan.merge, nil), failed)
		}
		return failed
	}
	switch {
	case plan.err != nil:
		return fail(fmt.Errorf("cannot delete %s: %w", plan.branch, plan.err))
	case plan.defaultBranch == "" || plan.branch == plan.defaultBranch:
		return fail(fmt.Errorf("not deleting %s: it is the default branch", plan.branch))
	case len(plan.stacked) >= maxStacked:
		return fail(fmt.Errorf("not deleting %s: %d or more PRs are based on it", plan.branch, maxStacked))
	}
	text := []string{plan.pr.GetTitle()}
	var nums []int
	if len(plan.stacked) > 0 {
		text = append(text, "", fmt.Sprintf("Moves onto %s first:", plan.base))
		for _, s := range plan.stacked {
			text = append(text, fmt.Sprintf("  #%d %s", s.Number, s.Title))
			nums = append(nums, s.Number)
		}
	}
	del := tasks.BranchDeletion{Branch: plan.branch, Base: plan.base, PRs: nums}
	if plan.merge != "" {
		return tasks.MergePR(m.ctx, plan.sid, plan.pr, plan.merge, &del)
	}
	m.dialog.Open(dialog.Spec{
		Title:   fmt.Sprintf("Delete branch %s of #%d?", plan.branch, plan.pr.GetNumber()),
		Text:    text,
		Buttons: []string{"Delete", "Cancel"},
		Submit: func(map[string]string, string) tea.Cmd {
			return tasks.DeleteBranch(m.ctx, plan.sid, plan.pr, del)
		},
	})
	return nil
}
