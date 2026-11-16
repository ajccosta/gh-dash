package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
)

// Dragging the edge between two column headers of a list (defaults.mouse)
// resizes the column, and a right click on it gives the column its configured
// width back. Widths are kept in layout.json (resize.go).

// colDrag is a column edge being dragged, from where it was picked up.
type colDrag struct {
	edge table.Edge
	x    int
}

type edgeFinder interface {
	EdgeAt(msg tea.MouseMsg) (table.Edge, bool)
}

func (m *Model) edgeAt(msg tea.MouseMsg) (table.Edge, bool) {
	if m.ctx.Config == nil || !m.ctx.Config.Defaults.Mouse {
		return table.Edge{}, false
	}
	if s, ok := m.getCurrSection().(edgeFinder); ok {
		return s.EdgeAt(msg)
	}
	return table.Edge{}, false
}

// columnEdgeRect is the column edge being dragged or under the mouse: the
// cell right of its line, which is tinted with the one left of it.
func (m *Model) columnEdgeRect() (x, y int, ok bool) {
	if !m.mouse.on {
		return 0, 0, false
	}
	if d := m.colDrag; d != nil {
		return m.mouse.x, d.edge.Y, true
	}
	e, ok := m.edgeAt(tea.MouseMotionMsg{X: m.mouse.x, Y: m.mouse.y})
	return e.X, e.Y, ok
}

// handleColumnMouse starts, follows and ends a drag of a column edge.
func (m *Model) handleColumnMouse(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		e, ok := m.edgeAt(msg)
		if !ok {
			return false, nil
		}
		switch msg.Button {
		case tea.MouseLeft:
			m.colDrag = &colDrag{edge: e, x: msg.X}
		case tea.MouseRight:
			delete(m.layout.Columns, e.Key)
			m.syncProgramContext()
			return true, m.saveLayout()
		}
		return true, nil
	case tea.MouseMotionMsg:
		if d := m.colDrag; d != nil {
			m.mouse = mousePos{x: msg.X, y: msg.Y, on: true}
			w := d.edge.Width + d.edge.Dir*(msg.X-d.x)
			m.layout.Columns[d.edge.Key] = min(max(w, table.MinColumnWidth), max(d.edge.Max, table.MinColumnWidth))
			m.syncProgramContext()
			return true, nil
		}
	case tea.MouseReleaseMsg:
		if m.colDrag != nil {
			m.colDrag = nil
			return true, m.saveLayout()
		}
	}
	return false, nil
}

func (m *Model) saveLayout() tea.Cmd {
	l := m.layout
	cols := make(map[string]int, len(l.Columns))
	for k, v := range l.Columns {
		cols[k] = v
	}
	l.Columns = cols // saved after this returns, while drags change the map
	return func() tea.Msg { data.SaveState(layoutFile, l); return nil }
}
