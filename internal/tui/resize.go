package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

// Dragging the line between the list and the preview (defaults.mouse)
// resizes them. The size is a fraction of the screen, like
// defaults.preview.width/height, and is kept in the state dir's layout.json,
// where it takes over from the config.

const layoutFile = "layout.json"

// layout is the size the preview was last dragged to (0: the config's).
type layout struct {
	PreviewWidth  float64 `json:"previewWidth,omitempty"`
	PreviewHeight float64 `json:"previewHeight,omitempty"`
}

const minPaneSize = 20 // cells either pane keeps while dragging

func (m *Model) previewWidth() float64 {
	if m.layout.PreviewWidth > 0 {
		return m.layout.PreviewWidth
	}
	return m.ctx.Config.Defaults.Preview.Width
}

func (m *Model) previewHeight() float64 {
	if m.layout.PreviewHeight > 0 {
		return m.layout.PreviewHeight
	}
	return m.ctx.Config.Defaults.Preview.Height
}

// onDivider reports whether (x, y) is on the line between list and preview.
func (m *Model) onDivider(x, y int) bool {
	if !m.sidebar.IsOpen || m.ctx.Config == nil || !m.ctx.Config.Defaults.Mouse {
		return false
	}
	top := common.HeaderHeight
	if m.ctx.PreviewPosition == "bottom" {
		return y == top+m.ctx.MainContentHeight
	}
	return x == m.ctx.MainContentWidth && y >= top && y < top+m.ctx.MainContentHeight
}

// dividerRect is the divider's cells, for the hover tint.
func (m *Model) dividerRect() (x0, y0, x1, y1 int) {
	top := common.HeaderHeight
	if m.ctx.PreviewPosition == "bottom" {
		y := top + m.ctx.MainContentHeight
		return 0, y, m.ctx.ScreenWidth - 1, y
	}
	return m.ctx.MainContentWidth, top, m.ctx.MainContentWidth, top + m.ctx.MainContentHeight - 1
}

// dragDivider moves the divider to the mouse.
func (m *Model) dragDivider(x, y int) {
	if m.ctx.PreviewPosition == "bottom" {
		avail := m.getBaseContentHeight() - m.ctx.Styles.Sidebar.BorderWidth
		h := avail - (y - common.HeaderHeight)
		h = min(max(h, 3), avail-3)
		m.layout.PreviewHeight = float64(h) / float64(avail)
	} else {
		w := m.ctx.ScreenWidth - x
		w = min(max(w, minPaneSize), m.ctx.ScreenWidth-minPaneSize)
		m.layout.PreviewWidth = float64(w) / float64(m.ctx.ScreenWidth)
	}
	m.syncMainContentDimensions()
	m.syncProgramContext()
	m.syncSidebar()
}

// handleDividerMouse starts, follows and ends a drag of the divider.
func (m *Model) handleDividerMouse(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && m.onDivider(msg.X, msg.Y) {
			m.dragging = true
			return true, nil
		}
	case tea.MouseMotionMsg:
		if m.dragging {
			m.mouse = mousePos{x: msg.X, y: msg.Y, on: true}
			m.dragDivider(msg.X, msg.Y)
			return true, nil
		}
	case tea.MouseReleaseMsg:
		if m.dragging {
			m.dragging = false
			l := m.layout
			return true, func() tea.Msg { data.SaveState(layoutFile, l); return nil }
		}
	}
	return false, nil
}
