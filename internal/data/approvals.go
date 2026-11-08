package data

import (
	"fmt"
	"sync"
)

// Local approvals: PRs you looked at and found good, GitHub not told (it
// won't let you approve your own). Each is kept with the head commit it was
// given at, in $XDG_STATE_HOME/gh-dash/approvals.json, so new commits need
// another look.

var (
	approvalsMu sync.Mutex
	approvals   map[string]string // "owner/repo#N" -> head commit
)

func approvalKey(repo string, number int) string { return fmt.Sprintf("%s#%d", repo, number) }

// LocalApproval is the head commit PR number of repo was approved locally
// at, or "".
func LocalApproval(repo string, number int) string {
	approvalsMu.Lock()
	defer approvalsMu.Unlock()
	if approvals == nil {
		approvals = map[string]string{}
		LoadState("approvals.json", &approvals)
	}
	return approvals[approvalKey(repo, number)]
}

// SetLocalApproval approves the PR locally at head, or takes the approval
// back when head is "".
func SetLocalApproval(repo string, number int, head string) {
	LocalApproval(repo, number) // loads them
	approvalsMu.Lock()
	defer approvalsMu.Unlock()
	if head == "" {
		delete(approvals, approvalKey(repo, number))
	} else {
		approvals[approvalKey(repo, number)] = head
	}
	SaveState("approvals.json", approvals)
}

// IsLocallyApproved reports whether pr is approved locally at its current head.
func (pr *PullRequestData) IsLocallyApproved() bool {
	if pr == nil || pr.HeadRefOid == "" {
		return false
	}
	return LocalApproval(pr.Repository.NameWithOwner, pr.Number) == pr.HeadRefOid
}
