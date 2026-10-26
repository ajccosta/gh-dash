package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

func scanZone(t *testing.T, view string, id string) *zone.ZoneInfo {
	t.Helper()
	zone.NewGlobal()
	zone.SetEnabled(true)
	zone.Scan(view)
	var z *zone.ZoneInfo
	require.Eventually(t, func() bool {
		z = zone.Get(id)
		return !z.IsZero()
	}, time.Second, 5*time.Millisecond)
	return z
}

func TestMouse_ViewSwitcherAndNewSectionButton(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.ScreenWidth = 120
	m.tabs.UpdateProgramContext(m.ctx)

	z := scanZone(t, m.tabs.View(), "view-tab-2")
	click := tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft}
	v, ok := m.tabs.ViewAt(click)
	require.True(t, ok)
	require.Equal(t, config.IssuesView, v)

	z = zone.Get("new-section")
	require.True(t, m.tabs.NewSectionAt(
		tea.MouseClickMsg{X: z.StartX, Y: z.StartY, Button: tea.MouseLeft}))
	require.False(t, m.tabs.NewSectionAt(
		tea.MouseClickMsg{X: 0, Y: z.StartY + 5, Button: tea.MouseLeft}))
}

func TestMouse_PreviewTabs(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	z := scanZone(t, m.prView.View(), "preview-tab-2")
	i := m.prView.TabAt(tea.MouseClickMsg{X: z.StartX, Y: z.StartY, Button: tea.MouseLeft})
	require.Equal(t, 2, i)
	m.prView.SelectTab(i)
	require.True(t, m.prView.IsCommitsTab())
}

func TestMouse_ClicksIgnoredUnlessEnabled(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.Config.Defaults.Mouse = false
	m.ctx.ScreenWidth = 120
	m.tabs.UpdateProgramContext(m.ctx)
	z := scanZone(t, m.tabs.View(), "view-tab-2")
	m.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	require.Equal(t, config.NotificationsView, m.ctx.View)
}

func TestMouse_CommitsAreClickable(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.prView.SelectTab(2)
	// Without enriched data there are no commits, so nothing is clickable.
	require.Equal(t, -1, m.prView.CommitAt(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft}))
}
