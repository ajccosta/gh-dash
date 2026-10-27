package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Hover feedback (defaults.mouse): the clickable thing under the mouse gets
// the theme's hover background. Everything clickable is a bubblezone mark, so
// this needs nothing from the components: a new mark is highlighted for free.

// hoveredZone is the smallest zone under the mouse, or nil.
func (m *Model) hoveredZone() *zone.ZoneInfo {
	if !m.mouse.on || zone.DefaultManager == nil {
		return nil
	}
	c := &zoneCollector{}
	zone.AnyInBounds(c, tea.MouseMotionMsg{X: m.mouse.x, Y: m.mouse.y})
	var best *zone.ZoneInfo
	for _, z := range c.zones {
		if best == nil || area(z) < area(best) {
			best = z
		}
	}
	return best
}

func area(z *zone.ZoneInfo) int { return (z.EndX - z.StartX + 1) * (z.EndY - z.StartY + 1) }

// paintHover gives the hovered zone's cells the hover background, keeping
// their own colours (and their own background where they set one).
func (m *Model) paintHover(view string) string {
	z := m.hoveredZone()
	if z == nil {
		return view
	}
	bg, _, _ := strings.Cut(lipgloss.NewStyle().Background(m.ctx.Theme.HoverBackground).Render("x"), "x")
	if bg == "" {
		return view
	}
	lines := strings.Split(view, "\n")
	for y := z.StartY; y <= z.EndY && y < len(lines); y++ {
		l := lines[y]
		w := ansi.StringWidth(l)
		if z.StartX >= w {
			continue
		}
		mid := ansi.Cut(l, z.StartX, z.EndX+1)
		for _, reset := range []string{"\x1b[m", "\x1b[0m", "\x1b[49m"} {
			mid = strings.ReplaceAll(mid, reset, reset+bg)
		}
		lines[y] = ansi.Truncate(l, z.StartX, "") + bg + mid + "\x1b[49m" + ansi.TruncateLeft(l, z.EndX+1, "")
	}
	return strings.Join(lines, "\n")
}

// mousePos is where the mouse last moved.
type mousePos struct {
	x, y int
	on   bool
}

// zoneCollector gathers the zones zone.AnyInBounds reports.
type zoneCollector struct{ zones []*zone.ZoneInfo }

func (c *zoneCollector) Init() tea.Cmd  { return nil }
func (c *zoneCollector) View() tea.View { return tea.NewView("") }
func (c *zoneCollector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if z, ok := msg.(zone.MsgZoneInBounds); ok {
		c.zones = append(c.zones, z.Zone)
	}
	return c, nil
}
