package table

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

func TestDraggedWidths(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	ctx := context.ProgramContext{Config: &cfg, Theme: theme.ParseTheme(&cfg)}
	ctx.Styles = context.InitStyles(ctx.Theme)
	ctx.Config.Theme.Ui.Table.Compact = true
	ctx.Config.Theme.Ui.Table.ShowSeparator = false
	ctx.ColumnWidths = map[string]int{"PR/Upd": 4, "PR/Title": 5, "Issue/Age": 20}
	cols := []Column{
		{Title: "Title", Grow: utils.BoolPtr(true)},
		{Title: "Upd", Width: utils.IntPtr(10)},
		{Title: "Age", Width: utils.IntPtr(6)},
	}
	m := NewModel(ctx, constants.Dimensions{Width: 60, Height: 10}, time.Time{}, time.Time{},
		cols, []Row{{"a title", "14m ago", "2h"}}, "PR", nil, "", false)
	m.SyncViewPortContent()

	// Upd is dragged; Age is another table's; Title grows whatever is kept.
	require.Equal(t, 4, m.Columns[1].ComputedWidth)
	require.Equal(t, 6, m.Columns[2].ComputedWidth)
	require.Equal(t, 50, m.Columns[0].ComputedWidth)

	// One line between Title and Upd (it resizes Upd) and one between Upd
	// and Age (it resizes Age); none on Title's left.
	require.Equal(t, 2, strings.Count(ansi.Strip(m.renderHeader()), "│"))

	// Drawing them keeps each escape sequence once (zone marks included).
	h := "\x1b[1m Ti \x1b[2z\x1b[3m Upd \x1b[2z\x1b[m"
	got, ok := putCell(h, 4, "|")
	require.True(t, ok)
	require.Equal(t, "\x1b[1m Ti \x1b[2z\x1b[3m|\x1b[m\x1b[1m\x1b[3mUpd \x1b[2z\x1b[m", got)
	_, ok = putCell(h, 2, "|")
	require.False(t, ok, "not over text")

	row := m.renderRow(0, m.renderHeaderColumns())
	require.Equal(t, 1, strings.Count(row, "\n")+1, "a narrowed cell is cut, not wrapped")
	require.Equal(t, 60, lipgloss.Width(row))
}
