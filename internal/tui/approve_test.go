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
	require.Equal(t, paneApprove, m.paneAbovePreview(), "↑ at the top of the preview focuses the button")
	m.setPane(paneApprove)
	require.True(t, m.prView.IsApproveFocused())

	cmd, handled := m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyDown})
	require.True(t, handled)
	require.Nil(t, cmd)
	require.Equal(t, panePreview, m.pane, "↓ goes back to the tabs")
	require.False(t, m.prView.IsApproveFocused())

	m.setPane(paneApprove)
	_, handled = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyUp})
	require.True(t, handled)
	require.Equal(t, paneSections, m.pane, "↑ goes on to the sections bar")

	m.setPane(paneApprove)
	_, handled = m.handleArrowNav(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, handled)
	require.True(t, m.prView.GetIsApproving(), "enter opens the approval prompt")
}
