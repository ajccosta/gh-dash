package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/footer"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issueview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/sidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tabs"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

// newApproveTestModel shows an open PR by "other" to the user "me" in the
// preview (as a PR notification, which needs no section rows). Nothing here
// runs gh: the approval prompt only opens, it is never submitted.
func newApproveTestModel(t *testing.T, author string) Model {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	ctx := &context.ProgramContext{
		Config:    &cfg,
		View:      config.NotificationsView,
		User:      "me",
		StartTask: func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sb := sidebar.NewModel()
	sb.UpdateProgramContext(ctx)
	sb.IsOpen = true
	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prView:           prview.NewModel(ctx),
		sidebar:          sb,
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}
	pr := &data.PullRequestData{State: "OPEN", Title: "a PR", Number: 1, Url: "https://x/1"}
	pr.Author.Login = author
	row := &prrow.Data{Primary: pr}
	m.notificationView.SetSubjectPR(row, "notif")
	m.prView.SetRow(row)
	m.prView.SetWidth(80)
	return m
}

func approveZone(t *testing.T, m Model) *zone.ZoneInfo {
	t.Helper()
	zone.NewGlobal()
	zone.SetEnabled(true)
	zone.Scan(m.prView.View())
	var z *zone.ZoneInfo
	require.Eventually(t, func() bool {
		z = zone.Get(prview.ApproveZoneId)
		return !z.IsZero()
	}, time.Second, 5*time.Millisecond)
	return z
}

func TestApproveButton_ClickOpensSamePromptAsKey(t *testing.T) {
	// The v key.
	byKey := newApproveTestModel(t, "other")
	byKey.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	require.True(t, byKey.prView.GetIsApproving(), "v opens the approval prompt")

	// A click on the button.
	m := newApproveTestModel(t, "other")
	z := approveZone(t, m)
	m.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	require.True(t, m.prView.GetIsApproving(), "clicking the button opens the approval prompt")
	require.Equal(t, byKey.prView.SelectedTab(), m.prView.SelectedTab())

	// A click elsewhere does nothing.
	m = newApproveTestModel(t, "other")
	z = approveZone(t, m)
	m.Update(tea.MouseClickMsg{X: z.EndX + 2, Y: z.StartY, Button: tea.MouseLeft})
	require.False(t, m.prView.GetIsApproving())
}

func TestApproveButton_HiddenForOwnPR(t *testing.T) {
	m := newApproveTestModel(t, "me")
	require.False(t, m.prView.CanApprove())
	require.NotContains(t, m.prView.View(), "Approve")
}

func TestApproveButton_ArrowNavigation(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.ctx.Config.Defaults.Preview.Navigation = "arrows"

	m.setPane(panePreview)
	require.Equal(t, paneButtons, m.paneAbovePreview(), "↑ at the top of the preview focuses the buttons")
	m.setPane(paneButtons)
	require.Equal(t, prview.ApproveZoneId, m.prView.FocusedButton(), "the first button is Approve")

	cmd, handled := m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyDown})
	require.True(t, handled)
	require.Nil(t, cmd)
	require.Equal(t, panePreview, m.pane, "↓ goes back to the tabs")
	require.Empty(t, m.prView.FocusedButton())

	m.setPane(paneButtons)
	_, handled = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyUp})
	require.True(t, handled)
	require.Equal(t, paneSections, m.pane, "↑ goes on to the sections bar")

	m.setPane(paneButtons)
	_, handled = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, handled)
	require.True(t, m.prView.GetIsApproving(), "enter opens the approval prompt")
}

func TestMergeButton_ClickAsksForConfirmation(t *testing.T) {
	m := newApproveTestModel(t, "me") // own PR: Merge is shown, Approve is not
	require.True(t, m.prView.CanMerge())
	require.False(t, m.prView.CanApprove())
	zone.NewGlobal()
	zone.SetEnabled(true)
	zone.Scan(m.prView.View())
	var z *zone.ZoneInfo
	require.Eventually(t, func() bool {
		z = zone.Get(prview.MergeZoneId)
		return !z.IsZero()
	}, time.Second, 5*time.Millisecond)

	m.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	require.True(t, m.notificationView.HasPendingAction(), "the click asks before merging")
	require.Equal(t, "pr_merge", m.notificationView.GetPendingAction())

	// Anything but y cancels; nothing runs.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	require.False(t, m.notificationView.HasPendingAction())
}

func TestMergeButton_ArrowNavigation(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.ctx.Config.Defaults.Preview.Navigation = "arrows"

	m.setPane(paneButtons)
	_, handled := m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyRight})
	require.True(t, handled)
	require.Equal(t, paneButtons, m.pane)
	require.Equal(t, prview.MergeZoneId, m.prView.FocusedButton(), "→ from Approve focuses Merge")

	_, handled = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.True(t, handled)
	require.Equal(t, prview.ApproveZoneId, m.prView.FocusedButton(), "← from Merge goes back to Approve")

	m.prView.SetFocusedButton(prview.MergeZoneId)
	_, _ = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Equal(t, panePreview, m.pane)

	// Own PR: no Approve, ↑ from the tabs lands on Merge, ← goes to the list.
	own := newApproveTestModel(t, "me")
	own.ctx.View = config.PRsView
	own.ctx.Config.Defaults.Preview.Navigation = "arrows"
	require.Equal(t, paneButtons, own.paneAbovePreview())
	own.setPane(paneButtons)
	require.Equal(t, prview.MergeZoneId, own.prView.FocusedButton())
	_, _ = own.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Equal(t, paneList, own.pane)
}

func TestCloseButton_ClickAsksForConfirmation(t *testing.T) {
	m := newApproveTestModel(t, "me")
	require.True(t, m.prView.CanClose())
	zone.NewGlobal()
	zone.SetEnabled(true)
	zone.Scan(m.prView.View())
	var z *zone.ZoneInfo
	require.Eventually(t, func() bool {
		z = zone.Get(prview.CloseZoneId)
		return !z.IsZero()
	}, time.Second, 5*time.Millisecond)

	m.Update(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	require.True(t, m.notificationView.HasPendingAction(), "the click asks before closing")
	require.Equal(t, "pr_close", m.notificationView.GetPendingAction())
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	require.False(t, m.notificationView.HasPendingAction())
}

func TestCloseButton_ArrowNavigation(t *testing.T) {
	m := newApproveTestModel(t, "other")
	m.ctx.View = config.PRsView
	m.ctx.Config.Defaults.Preview.Navigation = "arrows"

	m.prView.SetFocusedButton(prview.MergeZoneId)
	m.setPane(paneButtons)
	_, _ = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, prview.CloseZoneId, m.prView.FocusedButton(), "→ from Merge focuses Close")
	_, _ = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, prview.CloseZoneId, m.prView.FocusedButton(), "Close is the last button")
	_, _ = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Equal(t, prview.MergeZoneId, m.prView.FocusedButton())
}
