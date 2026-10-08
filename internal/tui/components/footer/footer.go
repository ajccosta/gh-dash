package footer

import (
	"fmt"
	"path"
	"strings"

	bbHelp "charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/git"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type Model struct {
	ctx             *context.ProgramContext
	leftSection     *string
	rightSection    *string
	help            bbHelp.Model
	ShowAll         bool
	ShowConfirmQuit bool
	prSelected      bool // a PR row is selected: hint at the approve key
}

func NewModel(ctx *context.ProgramContext) Model {
	help := bbHelp.New()
	help.ShowAll = true
	help.Styles = ctx.Styles.Help.BubbleStyles
	l := ""
	r := ""
	return Model{
		ctx:          ctx,
		help:         help,
		leftSection:  &l,
		rightSection: &r,
	}
}

func (m Model) View() string {
	var footer string

	if m.ShowConfirmQuit {
		footer = lipgloss.NewStyle().
			Render("Really quit? (Press y/enter to confirm, any other key to cancel)")
	} else {
		helpIndicator := lipgloss.NewStyle().
			Background(m.ctx.Theme.FaintText).
			Foreground(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render("? help")
		donationIndicator := zone.Mark("donate", lipgloss.NewStyle().
			Background(m.ctx.Theme.SelectedBackground).
			Foreground(m.ctx.Theme.WarningText).
			Padding(0, 1).
			Underline(true).
			Render(fmt.Sprintf("%s donate", constants.DonateIcon)))
		viewSwitcher := m.renderViewSwitcher(m.ctx)
		leftSection := ""
		if m.leftSection != nil {
			leftSection = *m.leftSection
		}
		rightSection := ""
		if m.rightSection != nil {
			rightSection = *m.rightSection
		}
		spacing := lipgloss.NewStyle().
			Background(m.ctx.Theme.SelectedBackground).
			Render(
				strings.Repeat(
					" ",
					utils.Max(0,
						m.ctx.ScreenWidth-lipgloss.Width(
							viewSwitcher,
						)-lipgloss.Width(leftSection)-
							lipgloss.Width(rightSection)-
							lipgloss.Width(
								helpIndicator,
							)-lipgloss.Width(donationIndicator),
					)))

		footer = m.ctx.Styles.Common.FooterStyle.
			Render(lipgloss.JoinHorizontal(lipgloss.Top, viewSwitcher, leftSection, spacing,
				rightSection, donationIndicator, helpIndicator))
	}

	if m.ShowAll {
		keymap := keys.CreateKeyMapForView(m.ctx.View)
		fullHelp := m.help.View(keymap)
		return lipgloss.JoinVertical(lipgloss.Top, footer, fullHelp)
	}

	return footer
}

func (m *Model) SetShowConfirmQuit(val bool) {
	m.ShowConfirmQuit = val
}

func (m *Model) SetWidth(width int) {
	m.help.SetWidth(width)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.help.Styles = ctx.Styles.Help.BubbleStyles
}

// viewHint tells how to switch views; the switcher itself is in the top bar.
func (m *Model) viewHint() string {
	hint := keys.SwitchViewKey(m.ctx.View) + " next view"
	if m.ctx.Config != nil && m.ctx.Config.Defaults.Preview.Navigation == "arrows" &&
		m.ctx.View != config.RepoView {
		hint += " · ↑ to top bar, ←/→ pick view"
	}
	if m.prSelected && m.ctx.View == config.PRsView {
		hint += " · " + keys.PRKeys.Approve.Help().Key + " approve"
	}
	return hint
}

// SetPRSelected tells the footer whether a PR row is selected.
func (m *Model) SetPRSelected(selected bool) {
	m.prSelected = selected
}

func (m *Model) renderViewSwitcher(ctx *context.ProgramContext) string {
	var repo string
	if m.ctx.RepoPath != "" {
		name := path.Base(m.ctx.RepoPath)
		if m.ctx.RepoUrl != "" {
			name = git.GetRepoShortName(m.ctx.RepoUrl)
		}
		repo = ctx.Styles.Common.FooterStyle.Render(fmt.Sprintf(" %s", name))
	}

	var user string
	if ctx.User != "" {
		user = ctx.Styles.Common.FooterStyle.Render("@" + ctx.User)
	}

	view := lipgloss.JoinHorizontal(
		lipgloss.Top,
		ctx.Styles.ViewSwitcher.InactiveView.Padding(0, 1).Render(m.viewHint()),
		lipgloss.NewStyle().Background(ctx.Styles.Common.FooterStyle.GetBackground()).Foreground(
			ctx.Styles.ViewSwitcher.ViewsSeparator.GetBackground()).Render("▌ "),
		repo,
		ctx.Styles.Common.FooterStyle.Foreground(m.ctx.Theme.FaintText).Render(" • "),
		user,
		ctx.Styles.Common.FooterStyle.Foreground(m.ctx.Theme.FaintBorder).Render(" │"),
	)

	return ctx.Styles.ViewSwitcher.Root.Render(view)
}

func (m *Model) SetLeftSection(leftSection string) {
	*m.leftSection = leftSection
}

func (m *Model) SetRightSection(rightSection string) {
	*m.rightSection = rightSection
}
