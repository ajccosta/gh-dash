package table

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Dragging the line between two header cells resizes a column. The growing
// column (the title) takes up the difference, so the edge resizes the column
// on the far side from it: the one on its left before the title, the one on
// its right after. Widths are kept in ctx.ColumnWidths by ColumnKey, so every
// section showing the same kind of item gets them.

// HeaderRowZone marks the header row, to find the edges in it; it is not a
// clickable thing of its own, so it gets no hover tint.
const HeaderRowZone = "table-header-row"

// MinColumnWidth and minGrowWidth are the narrowest a dragged column and the
// growing column get.
const (
	MinColumnWidth = 3
	minGrowWidth   = 10
)

// Edge is a column edge under the mouse: dragging it right by n cells makes
// the column Dir*n wider, up to Max.
type Edge struct {
	Key        string
	Width, Max int
	Dir        int
	X, Y       int // the screen cell right of the line
}

// ColumnKey names column i for its dragged width: the table's item type and
// the column's title (its index when it has none).
func (m *Model) ColumnKey(i int) string {
	t := m.Columns[i].Title
	if t == "" {
		t = "#" + strconv.Itoa(i)
	}
	return m.name + "/" + t
}

func grows(c Column) bool { return c.Grow != nil && *c.Grow }

// edges are the lines between the header cells that can be dragged, X
// counted from the header's left.
func (m *Model) edges(cells []string) []Edge {
	var idx []int // m.Columns index of each shown column
	for i, c := range m.Columns {
		if c.Hidden == nil || !*c.Hidden {
			idx = append(idx, i)
		}
	}
	g, growW := len(cells), 0
	for j, i := range idx {
		if grows(m.Columns[i]) {
			g, growW = j, lipgloss.Width(cells[j])
			break
		}
	}
	var out []Edge
	edge := 0
	for j := 0; j+1 < len(cells); j++ {
		edge += lipgloss.Width(cells[j])
		t, dir := j, 1
		if j >= g {
			t, dir = j+1, -1
		}
		if grows(m.Columns[idx[t]]) {
			continue
		}
		w := lipgloss.Width(cells[t])
		max := w + 200
		if g < len(cells) {
			max = w + growW - minGrowWidth
		}
		out = append(out, Edge{Key: m.ColumnKey(idx[t]), Width: w, Max: max, Dir: dir, X: edge})
	}
	return out
}

// EdgeAt returns the column edge under the mouse, on the header row.
func (m *Model) EdgeAt(msg tea.MouseMsg) (Edge, bool) {
	if zone.DefaultManager == nil {
		return Edge{}, false
	}
	z := zone.Get(HeaderRowZone)
	if !z.InBounds(msg) {
		return Edge{}, false
	}
	x := msg.Mouse().X - z.StartX
	for _, e := range m.edges(m.renderHeaderColumns()) {
		if x == e.X-1 || x == e.X {
			e.X, e.Y = z.StartX+e.X, msg.Mouse().Y
			return e, true
		}
	}
	return Edge{}, false
}

// drawEdges puts a faint │ on each line that can be dragged, in the blank
// cell beside it, so the header shows where the columns can be resized.
func (m *Model) drawEdges(header string, cells []string) string {
	bar := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render("│")
	for _, e := range m.edges(cells) {
		for _, x := range []int{e.X - 1, e.X} {
			if s, ok := putCell(header, x, bar); ok {
				header = s
				break
			}
		}
	}
	return header
}

// putCell replaces the blank at cell x of s with c, and restores the colours
// s had there after it. It leaves every escape sequence of s in place once
// (ansi.Truncate and TruncateLeft both keep the ones around the cut, so
// splicing with them repeats the zone marks, which slows every frame).
func putCell(s string, x int, c string) (string, bool) {
	var sgr []string // the colours set since the last reset
	col := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			n := escLen(s[i:])
			if seq := s[i : i+n]; strings.HasSuffix(seq, "m") && strings.HasPrefix(seq, "\x1b[") {
				if seq == "\x1b[m" || seq == "\x1b[0m" {
					sgr = sgr[:0]
				} else {
					sgr = append(sgr, seq)
				}
			}
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if col == x {
			if r != ' ' {
				return s, false
			}
			return s[:i] + c + "\x1b[m" + strings.Join(sgr, "") + s[i+size:], true
		}
		col += ansi.StringWidth(string(r))
		i += size
	}
	return s, false
}

// escLen is the length of the escape sequence s starts with: a CSI (ESC [
// … final byte), an OSC (ESC ] … BEL or ESC \), or ESC and one byte.
func escLen(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[':
		for j := 2; j < len(s); j++ {
			if s[j] >= 0x40 && s[j] <= 0x7e {
				return j + 1
			}
		}
	case ']':
		for j := 2; j < len(s); j++ {
			if s[j] == '\a' {
				return j + 1
			}
			if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
		}
	default:
		return 2
	}
	return len(s)
}
