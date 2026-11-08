package tasks

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type SectionIdentifier struct {
	Id   int
	Type string
}

type UpdatePRMsg struct {
	PrNumber       int
	IsClosed       *bool
	NewComment     *data.Comment
	ReadyForReview *bool
	IsMerged       *bool
	BranchDeleted  *bool
	// Rebased: other PRs of Repo (by number) now based on another branch.
	Repo             string
	Rebased          map[int]string
	AddedAssignees   *data.Assignees
	RemovedAssignees *data.Assignees
	Labels           *data.PRLabels
}

type UpdateBranchMsg struct {
	Name      string
	IsCreated *bool
	NewPr     *data.PullRequestData
}

func buildTaskId(prefix string, prNumber int) string {
	return fmt.Sprintf("%s_%d", prefix, prNumber)
}

type GitHubTask struct {
	Id           string
	Args         []string
	Section      SectionIdentifier
	StartText    string
	FinishedText string
	FailedText   string     // prefix of the error shown when gh fails; optional
	More         [][]string // further gh calls, run in order after Args while each succeeds
	Msg          func(c *exec.Cmd, err error) tea.Msg
}

func fireTask(ctx *context.ProgramContext, task GitHubTask) tea.Cmd {
	start := context.Task{
		Id:           task.Id,
		StartText:    task.StartText,
		FinishedText: task.FinishedText,
		State:        context.TaskStart,
		Error:        nil,
	}

	startCmd := ctx.StartTask(start)
	return tea.Batch(startCmd, func() tea.Msg {
		log.Info("Running task", "cmd", "gh "+strings.Join(task.Args, " "))
		c := exec.Command("gh", task.Args...)
		var stderr bytes.Buffer
		c.Stderr = &stderr

		err := ghError(c.Run(), stderr.String(), task.FailedText)
		for _, args := range task.More {
			if err != nil {
				break
			}
			log.Info("Running task", "cmd", "gh "+strings.Join(args, " "))
			c = exec.Command("gh", args...)
			stderr.Reset()
			c.Stderr = &stderr
			err = ghError(c.Run(), stderr.String(), task.FailedText)
		}
		return constants.TaskFinishedMsg{
			TaskId:      task.Id,
			SectionId:   task.Section.Id,
			SectionType: task.Section.Type,
			Err:         err,
			Msg:         task.Msg(c, err),
		}
	})
}

// NextMsg has the dashboard run what Run returns: the task that comes after
// another, started only once that one is done.
type NextMsg struct{ Run func() tea.Cmd }

// Then runs step in the background, shown as a task of its own, and next once
// it succeeds: something to do in git, say, before the gh calls next makes.
func Then(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData,
	id, startText, finishedText string, step func() error, next func() tea.Cmd,
) tea.Cmd {
	task := context.Task{Id: buildTaskId(id, pr.GetNumber()), StartText: startText,
		FinishedText: finishedText, State: context.TaskStart}
	return tea.Batch(ctx.StartTask(task), func() tea.Msg {
		log.Info("Running task", "id", task.Id)
		err := step()
		done := func() tea.Msg {
			return constants.TaskFinishedMsg{TaskId: task.Id, SectionId: section.Id,
				SectionType: section.Type, Err: err, Msg: UpdatePRMsg{PrNumber: pr.GetNumber()}}
		}
		if err != nil {
			return done()
		}
		return tea.BatchMsg{done, func() tea.Msg { return NextMsg{next} }}
	})
}

func OpenBranchPR(ctx *context.ProgramContext, section SectionIdentifier, branch string) tea.Cmd {
	return fireTask(ctx, GitHubTask{
		Id: fmt.Sprintf("branch_open_%s", branch),
		Args: []string{
			"pr",
			"view",
			"--web",
			branch,
			"-R",
			ctx.RepoUrl,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Opening PR for branch %s", branch),
		FinishedText: fmt.Sprintf("PR for branch %s has been opened", branch),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{}
		},
	})
}

func ReopenPR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_reopen", prNumber),
		Args: []string{
			"pr",
			"reopen",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Reopening PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been reopened", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsClosed: utils.BoolPtr(false),
			}
		},
	})
}

func ClosePR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_close", prNumber),
		Args: []string{
			"pr",
			"close",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Closing PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been closed", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsClosed: utils.BoolPtr(true),
			}
		},
	})
}

func PRReady(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_ready", prNumber),
		Args: []string{
			"pr",
			"ready",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Marking PR #%d as ready for review", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been marked as ready for review", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber:       prNumber,
				ReadyForReview: utils.BoolPtr(true),
			}
		},
	})
}

// BranchDeletion is the head branch of a merged PR to delete, once the open
// PRs based on it are moved onto Base. GitHub closes those PRs when their
// base branch goes, so it is never deleted with them still on it (as
// `gh pr merge --delete-branch` does).
type BranchDeletion struct {
	Branch, Base string
	PRs          []int
}

// steps moves the PRs, then deletes the branch.
func (d BranchDeletion) steps(repo string) [][]string {
	var steps [][]string
	for _, n := range d.PRs {
		steps = append(steps, []string{"pr", "edit", fmt.Sprint(n), "-R", repo, "--base", d.Base})
	}
	segments := strings.Split(d.Branch, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return append(steps, []string{"api", "-X", "DELETE",
		"repos/" + repo + "/git/refs/heads/" + strings.Join(segments, "/")})
}

// done tells msg whether the branch went, and with it the PRs that moved.
// On a failure some PRs may have moved already: the next refresh shows them.
func (d BranchDeletion) done(msg *UpdatePRMsg, repo string, deleted bool) {
	msg.BranchDeleted = &deleted
	if deleted {
		msg.Repo, msg.Rebased = repo, map[int]string{}
		for _, n := range d.PRs {
			msg.Rebased[n] = d.Base
		}
	}
}

// MergePR runs `gh pr merge` in the background with the method ("merge",
// "squash" or "rebase") chosen in the merge dialog, then the branch deletion
// if one is given.
func MergePR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	method string,
	del *BranchDeletion,
) tea.Cmd {
	prNumber := pr.GetNumber()
	repo := pr.GetRepoNameWithOwner()
	args := []string{"pr", "merge", fmt.Sprint(prNumber), "-R", repo, "--" + method}
	task := GitHubTask{
		Id:           buildTaskId("merge", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Merging #%d (%s)", prNumber, method),
		FinishedText: fmt.Sprintf("Merged #%d", prNumber),
		FailedText:   fmt.Sprintf("Merge #%d failed", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			isMerged := err == nil || !slices.Equal(c.Args[1:], args)
			msg := UpdatePRMsg{PrNumber: prNumber, IsMerged: &isMerged}
			if del != nil && isMerged {
				del.done(&msg, repo, err == nil)
			}
			return msg
		},
	}
	if del != nil {
		task.More = del.steps(repo)
		task.StartText = fmt.Sprintf("Merging #%d (%s), then deleting %s", prNumber, method, del.Branch)
		task.FinishedText = fmt.Sprintf("Merged #%d and deleted %s", prNumber, del.Branch)
	}
	return fireTask(ctx, task)
}

// DeleteBranch deletes the head branch of the merged PR pr, as del says. It
// stops at the first failure, so the branch stays while a PR still points at it.
func DeleteBranch(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	del BranchDeletion,
) tea.Cmd {
	repo := pr.GetRepoNameWithOwner()
	steps := del.steps(repo)
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id:           buildTaskId("delete_branch", prNumber),
		Args:         steps[0],
		More:         steps[1:],
		Section:      section,
		StartText:    fmt.Sprintf("Deleting %s", del.Branch),
		FinishedText: fmt.Sprintf("Deleted %s", del.Branch),
		FailedText:   fmt.Sprintf("Deleting %s failed", del.Branch),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			msg := UpdatePRMsg{PrNumber: prNumber}
			del.done(&msg, repo, err == nil)
			return msg
		},
	})
}

func CreatePR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	branchName string,
	title string,
) tea.Cmd {
	c := exec.Command(
		"gh",
		"pr",
		"create",
		"--title",
		title,
		"-R",
		ctx.RepoUrl,
	)

	taskId := fmt.Sprintf("create_pr_%s", title)
	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf(`Creating PR "%s"`, title),
		FinishedText: fmt.Sprintf(`PR "%s" has been created`, title),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := ctx.StartTask(task)

	return tea.Batch(startCmd, tea.ExecProcess(c, func(err error) tea.Msg {
		isCreated := err == nil && c.ProcessState.ExitCode() == 0

		return constants.TaskFinishedMsg{
			SectionId:   section.Id,
			SectionType: section.Type,
			TaskId:      taskId,
			Err:         nil,
			Msg:         UpdateBranchMsg{Name: branchName, IsCreated: &isCreated},
		}
	}))
}

func updatePRTask(section SectionIdentifier, pr data.RowData) GitHubTask {
	prNumber := pr.GetNumber()
	return GitHubTask{
		Id: buildTaskId("pr_update", prNumber),
		Args: []string{
			"pr",
			"update-branch",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Updating PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been updated", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
			}
		},
	}
}

func UpdatePR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	return fireTask(ctx, updatePRTask(section, pr))
}

func AssignPR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	usernames []string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	args := []string{
		"pr",
		"edit",
		fmt.Sprint(prNumber),
		"-R",
		pr.GetRepoNameWithOwner(),
	}
	for _, assignee := range usernames {
		args = append(args, "--add-assignee", assignee)
	}
	return fireTask(ctx, GitHubTask{
		Id:           buildTaskId("pr_assign", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Assigning pr #%d to %s", prNumber, usernames),
		FinishedText: fmt.Sprintf("pr #%d has been assigned to %s", prNumber, usernames),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			returnedAssignees := data.Assignees{Nodes: []data.Assignee{}}
			for _, assignee := range usernames {
				returnedAssignees.Nodes = append(
					returnedAssignees.Nodes,
					data.Assignee{Login: assignee},
				)
			}
			return UpdatePRMsg{
				PrNumber:       prNumber,
				AddedAssignees: &returnedAssignees,
			}
		},
	})
}

func UnassignPR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	usernames []string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	args := []string{
		"pr",
		"edit",
		fmt.Sprint(prNumber),
		"-R",
		pr.GetRepoNameWithOwner(),
	}
	for _, assignee := range usernames {
		args = append(args, "--remove-assignee", assignee)
	}
	return fireTask(ctx, GitHubTask{
		Id:           buildTaskId("pr_unassign", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Unassigning %s from pr #%d", usernames, prNumber),
		FinishedText: fmt.Sprintf("%s unassigned from pr #%d", usernames, prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			returnedAssignees := data.Assignees{Nodes: []data.Assignee{}}
			for _, assignee := range usernames {
				returnedAssignees.Nodes = append(
					returnedAssignees.Nodes,
					data.Assignee{Login: assignee},
				)
			}
			return UpdatePRMsg{
				PrNumber:         prNumber,
				RemovedAssignees: &returnedAssignees,
			}
		},
	})
}

func CommentOnPR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	body string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_comment", prNumber),
		Args: []string{
			"pr",
			"comment",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
			"-b",
			body,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Commenting on PR #%d", prNumber),
		FinishedText: fmt.Sprintf("Commented on PR #%d", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				NewComment: &data.Comment{
					Author:    struct{ Login string }{Login: ctx.User},
					Body:      body,
					UpdatedAt: time.Now(),
				},
			}
		},
	})
}

func ApprovePR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	comment string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	args := []string{
		"pr",
		"review",
		"-R",
		pr.GetRepoNameWithOwner(),
		fmt.Sprint(prNumber),
		"--approve",
	}
	if comment != "" {
		args = append(args, "--body", comment)
	}
	return fireTask(ctx, GitHubTask{
		Id:           buildTaskId("pr_approve", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Approving #%d", prNumber),
		FinishedText: fmt.Sprintf("Approved #%d", prNumber),
		FailedText:   fmt.Sprintf("Approve #%d failed", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
			}
		},
	})
}

func ApproveWorkflows(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
) tea.Cmd {
	prNumber := pr.GetNumber()
	repo := pr.GetRepoNameWithOwner()
	taskId := buildTaskId("pr_approve_workflows", prNumber)

	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Approving workflows for PR #%d", prNumber),
		FinishedText: fmt.Sprintf("Workflows for PR #%d have been approved", prNumber),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := ctx.StartTask(task)

	return tea.Batch(startCmd, func() tea.Msg {
		// Step 1: Get head SHA
		shaCmd := exec.Command("gh", "pr", "view", fmt.Sprint(prNumber),
			"-R", repo, "--json", "headRefOid", "--jq", ".headRefOid")
		shaOut, err := shaCmd.Output()
		if err != nil {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("failed to get head SHA: %w", err),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}
		sha := strings.TrimSpace(string(shaOut))

		// Step 2: Get workflow run IDs awaiting approval
		runsCmd := exec.Command("gh", "api",
			fmt.Sprintf("repos/%s/actions/runs?status=action_required&head_sha=%s", repo, sha),
			"--jq", ".workflow_runs[].id")
		runsOut, err := runsCmd.Output()
		if err != nil {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("failed to get workflow runs: %w", err),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}

		runIds := strings.Fields(strings.TrimSpace(string(runsOut)))
		if len(runIds) == 0 {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("no workflows awaiting approval"),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}

		// Step 3: Approve each run (best-effort)
		var lastErr error
		approved := 0
		for _, runId := range runIds {
			log.Info("Approving workflow run", "runId", runId, "pr", prNumber)
			approveCmd := exec.Command("gh", "api", "-X", "POST",
				fmt.Sprintf("repos/%s/actions/runs/%s/approve", repo, runId))
			output, err := approveCmd.CombinedOutput()
			if err != nil {
				outStr := string(output)
				if strings.Contains(outStr, "not from a fork pull request") {
					lastErr = fmt.Errorf(
						"workflow not approvable via API (only fork PR workflows can be approved)",
					)
				} else {
					lastErr = fmt.Errorf("failed to approve run %s: %w", runId, err)
				}
			} else {
				approved++
			}
		}

		return constants.TaskFinishedMsg{
			TaskId:      taskId,
			SectionId:   section.Id,
			SectionType: section.Type,
			Err:         lastErr,
			Msg:         UpdatePRMsg{PrNumber: prNumber},
		}
	})
}
