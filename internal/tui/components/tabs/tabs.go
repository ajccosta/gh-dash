package tabs

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/carousel"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type SectionTab struct {
	section section.Section
	spinner spinner.Model
}

type Model struct {
	sections      []section.Section
	sectionTabs   []SectionTab
	carousel      carousel.Model
	ctx           *context.ProgramContext
	latestVersion string
	viewsFocused  bool
}

// Views are the views the view switcher offers, in the order the switch key cycles them.
var Views = []config.ViewType{config.NotificationsView, config.PRsView, config.IssuesView}

var viewTitles = map[config.ViewType]string{
	config.NotificationsView: "Notifications",
	config.PRsView:           "PRs",
	config.IssuesView:        "Issues",
}

func NewModel(ctx *context.ProgramContext) Model {
	c := carousel.NewModel(
		carousel.WithHeight(1),
		carousel.WithOverflowIndicators("←", "→"),
		carousel.WithSeparators(),
	)
	m := Model{
		carousel: c,
	}
	m.UpdateProgramContext(ctx)

	return m
}

func (m Model) Init() tea.Cmd {
	return m.fetchHasNewVersion()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	switch msg := msg.(type) {
	case latestVersionMsg:
		m.latestVersion = msg.version
	case spinner.TickMsg:
		for i, tab := range m.sectionTabs {
			if tab.section.GetIsLoading() {
				var cmd tea.Cmd
				m.sectionTabs[i].spinner, cmd = tab.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	m.UpdateTabTitles()

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	crsl := m.carousel.View()
	logo := m.viewLogo()
	return m.ctx.Styles.Tabs.TabsRow.
		Width(m.ctx.ScreenWidth).
		Height(common.HeaderHeight).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, m.viewViewSwitcher(), crsl,
			m.viewNewSectionButton(), logo))
}

type latestVersionMsg struct {
	version string
	err     error
}

func (m Model) viewNewSectionButton() string {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(m.ctx.Styles.Tabs.TabSeparator.GetForeground()).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().
					Foreground(m.ctx.Styles.Colors.SuccessText).
					Render("+ "),
				keys.Keys.NewSection.Help().Key,
			))
}

// viewViewSwitcher renders "Notifications │ PRs │ Issues  s" with the current
// view highlighted (reverse video while the switcher has focus) and the switch key.
func (m Model) viewViewSwitcher() string {
	items := make([]string, 0, 2*len(Views))
	for i, v := range Views {
		if i > 0 {
			items = append(items, m.ctx.Styles.Tabs.TabSeparator.Render("│"))
		}
		style := m.ctx.Styles.Tabs.Tab
		if v == m.ctx.View {
			style = m.ctx.Styles.Tabs.ActiveTab
			if m.viewsFocused {
				style = style.Reverse(true)
			}
		}
		items = append(items, style.Padding(0, 1).Render(viewTitles[v]))
	}
	items = append(items, m.ctx.Styles.Common.FaintTextStyle.PaddingLeft(1).
		Render(keys.SwitchViewKey(m.ctx.View)))
	return lipgloss.NewStyle().
		Padding(0, 1, 0, 0).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(m.ctx.Styles.Tabs.TabSeparator.GetForeground()).
		Render(lipgloss.JoinHorizontal(lipgloss.Top, items...))
}

func (m Model) carouselWidth() int {
	logo := m.viewLogo()
	newSectionButton := m.viewNewSectionButton()
	return m.ctx.ScreenWidth - lipgloss.Width(logo) -
		lipgloss.Width(newSectionButton) - lipgloss.Width(m.viewViewSwitcher())
}

func (m *Model) fetchHasNewVersion() tea.Cmd {
	return func() tea.Msg {
		r, err := data.FetchLatestVersion()
		return latestVersionMsg{
			version: r.Repository.LatestRelease.TagName,
			err:     err,
		}
	}
}

func (m *Model) CurrSectionId() int {
	return m.carousel.Cursor()
}

// SetFocused highlights the current section tab while the bar has focus.
func (m *Model) SetFocused(focused bool) {
	m.carousel.SetHighlighted(focused)
}

// SetViewsFocused highlights the current view in the view switcher.
func (m *Model) SetViewsFocused(focused bool) {
	m.viewsFocused = focused
}

func (m *Model) SetCurrSectionId(id int) {
	m.carousel.SetCursor(id)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.carousel.SetStyles(carousel.Styles{
		Item:              ctx.Styles.Tabs.Tab,
		Selected:          ctx.Styles.Tabs.ActiveTab,
		OverflowIndicator: ctx.Styles.Tabs.OverflowIndicator,
		Separator:         ctx.Styles.Tabs.TabSeparator,
	})

	m.carousel.SetWidth(m.carouselWidth())
}

func (m *Model) SetSections(sections []section.Section) {
	sectionTabs := make([]SectionTab, 0)
	for _, s := range sections {
		tab := SectionTab{section: s, spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot), spinner.WithStyle(
				lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2)))}
		sectionTabs = append(sectionTabs, tab)
	}
	m.sectionTabs = sectionTabs
	m.UpdateTabTitles()
}

func (m *Model) UpdateTabTitles() {
	titles := make([]string, 0)
	for i, tab := range m.sectionTabs {
		cfg := tab.section.GetConfig()
		title := cfg.Title
		if tab.section.GetIsLoading() {
			title = fmt.Sprintf("%s %s", title, m.sectionTabs[i].spinner.View())
		} else if m.ctx.Config.Theme.Ui.SectionsShowCount {
			title = fmt.Sprintf("%s (%s)", title,
				utils.ShortNumber(tab.section.GetTotalCount()))
		}

		titles = append(titles, title)
	}

	oldCursor := m.carousel.Cursor()
	m.carousel.SetItems(titles)
	m.carousel.SetCursor(oldCursor)
}

func (m *Model) viewLogo() string {
	version := lipgloss.NewStyle().Foreground(m.ctx.Theme.SecondaryText).Render(m.ctx.Version)
	if m.latestVersion != "" && m.ctx.Version != "dev" && m.ctx.Version != m.latestVersion {
		version = lipgloss.JoinHorizontal(
			lipgloss.Top,
			version,
			lipgloss.NewStyle().
				Foreground(m.ctx.Styles.Colors.SuccessText).
				Render(" ↑ Update available!"),
		)
	}

	return lipgloss.NewStyle().
		Margin(0, 1, 0, 2).
		Render(lipgloss.JoinHorizontal(
			lipgloss.Bottom,
			lipgloss.NewStyle().
				Background(lipgloss.Darken(context.LogoColor, 0.7)).
				Padding(0, 1).
				Foreground(context.LogoColor).
				Bold(true).
				Render("DASH"),
			" ",
			version,
		))
}

func (m *Model) SetAllLoading() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	for i := range m.sectionTabs {
		cmds = append(cmds, m.sectionTabs[i].spinner.Tick)
	}

	return cmds
}
