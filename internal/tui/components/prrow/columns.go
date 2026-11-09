package prrow

import (
	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

// Column is a column of the PR list: its header, which the section builds
// (GetSectionColumns), and its cell, which ToTableRow renders. A new column is
// one entry in Columns.
type Column struct {
	// Layout picks its layout from the defaults' or a section's; nil: none.
	Layout func(config.PrsLayoutConfig) config.ColumnConfig
	// Header is the column, with its layout.
	Header func(ctx *context.ProgramContext, l config.ColumnConfig) table.Column
	// Sort is its sort key, and SortTitle its title in full; "": it does not
	// sort.
	Sort, SortTitle string
	CompactOnly     bool // only in the compact table
	Cell            func(pr *PullRequest, selected bool) string
}

// StackTitle heads the stack graph's column.
const StackTitle = "Stack"

// Columns are the PR list's columns, in order.
var Columns = []Column{
	{
		// the stack graph, shown only in stack order
		Header: func(*context.ProgramContext, config.ColumnConfig) table.Column {
			return table.Column{Title: StackTitle, Width: utils.IntPtr(len(StackTitle) + 2), Hidden: utils.BoolPtr(true)}
		},
		Cell: (*PullRequest).renderGraph,
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.State },
		Header: func(_ *context.ProgramContext, l config.ColumnConfig) table.Column {
			return table.Column{Width: utils.IntPtr(6), Hidden: l.Hidden} // the PR number, "#1234"
		},
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderState() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Repo },
		Header: fixed("Repo"), Sort: "repo", SortTitle: "Repo", CompactOnly: true,
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderRepoName() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Title },
		Header: func(_ *context.ProgramContext, l config.ColumnConfig) table.Column {
			return table.Column{Title: "Title", Grow: utils.BoolPtr(true), Hidden: l.Hidden}
		},
		Sort: "title", SortTitle: "Title",
		Cell: func(pr *PullRequest, selected bool) string {
			if pr.Ctx.Config.Theme.Ui.Table.Compact {
				return pr.renderTitle() // the repo and the author have columns of their own
			}
			return pr.renderExtendedTitle(selected)
		},
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Author },
		Header: fixed("Author"), Sort: "author", SortTitle: "Author", CompactOnly: true,
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderAuthor() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Labels },
		Header: fixed(constants.LabelsIcon),
		Cell:   (*PullRequest).renderLabels,
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Assignees },
		Header: fixed("Assignees"),
		Cell:   func(pr *PullRequest, _ bool) string { return pr.renderAssignees() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Base },
		Header: fixed("Base"), Sort: "base", SortTitle: "Base",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderBaseName() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.NumComments },
		Header: width(constants.CommentsIcon, 4), Sort: "comments", SortTitle: "Comments",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderNumComments() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.ReviewStatus },
		Header: width("Rv", 4), Sort: "review", SortTitle: "Review",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderReviewStatus() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Ci },
		Header: func(ctx *context.ProgramContext, l config.ColumnConfig) table.Column {
			return table.Column{Title: "CI", Width: &ctx.Styles.PrSection.CiCellWidth, Grow: new(bool), Hidden: l.Hidden}
		},
		Sort: "checks", SortTitle: "Checks",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderCiStatus() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Conflicts },
		Header: fixed("Conflicts"),
		Cell:   func(pr *PullRequest, _ bool) string { return pr.renderConflicts() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.Lines },
		Header: fixed("±"), Sort: "lines", SortTitle: "±",
		Cell: (*PullRequest).RenderLines,
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.UpdatedAt },
		Header: fixed("Upd"), Sort: "updated", SortTitle: "Updated",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderUpdateAt() },
	},
	{
		Layout: func(l config.PrsLayoutConfig) config.ColumnConfig { return l.CreatedAt },
		Header: fixed("Age"), Sort: "created", SortTitle: "Age",
		Cell: func(pr *PullRequest, _ bool) string { return pr.renderCreatedAt() },
	},
}

// fixed is a column as wide as its layout says.
func fixed(title string) func(*context.ProgramContext, config.ColumnConfig) table.Column {
	return func(_ *context.ProgramContext, l config.ColumnConfig) table.Column {
		return table.Column{Title: title, Width: l.Width, Hidden: l.Hidden}
	}
}

// width is a column of width w, whatever its layout says.
func width(title string, w int) func(*context.ProgramContext, config.ColumnConfig) table.Column {
	return func(_ *context.ProgramContext, l config.ColumnConfig) table.Column {
		return table.Column{Title: title, Width: utils.IntPtr(w), Hidden: l.Hidden}
	}
}

// Shown tells whether the column is in the table, compact or not.
func (c Column) Shown(compact bool) bool {
	return compact || !c.CompactOnly
}

func (pr *PullRequest) ToTableRow(isSelected bool) table.Row {
	var row table.Row
	for _, c := range Columns {
		if c.Shown(pr.Ctx.Config.Theme.Ui.Table.Compact) {
			row = append(row, c.Cell(pr, isSelected))
		}
	}
	return row
}
