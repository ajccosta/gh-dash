package prrow

import (
	"time"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

type Data struct {
	Primary    *data.PullRequestData
	Enriched   data.EnrichedPullRequestData
	IsEnriched bool
	// StackLanes: with the list in stack order, the lines of the stack graph
	// left of its node, on the PR's own line and on the one below it.
	StackLanes [2]string
	// Extra: no result of the section's search, only shown in stack order
	// because PRs of the list are stacked on it.
	Extra bool
}

func (data Data) GetTitle() string {
	return data.Primary.Title
}

func (data Data) GetRepoNameWithOwner() string {
	return data.Primary.Repository.NameWithOwner
}

func (data Data) GetNumber() int {
	return data.Primary.Number
}

func (data Data) GetUrl() string {
	return data.Primary.Url
}

func (data Data) GetUpdatedAt() time.Time {
	return data.Primary.UpdatedAt
}

func (data Data) GetCreatedAt() time.Time {
	return data.Primary.CreatedAt
}
