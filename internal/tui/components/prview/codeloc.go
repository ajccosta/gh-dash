package prview

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// A review comment's place in the code. The Activity tab shows it as
// "path#L12-L15" over the lines it is about (from the comment's diff hunk);
// clicking it opens the PR diff there, with GHDIFF_GOTO telling ghdiff which
// lines to scroll to and highlight.

// CodeLoc is the code a review thread comments on.
type CodeLoc struct {
	Path       string
	Side       string // RIGHT (the new code) or LEFT (the old)
	Start, End int    // lines on Side, in the PR's current diff; 0 when outdated
	Outdated   bool

	hunk               string // the diff hunk up to the commented line
	origStart, origEnd int    // lines on Side when the comment was made
}

// GotoEnv is the environment entry that makes ghdiff open at these lines.
func (l CodeLoc) GotoEnv() string {
	start, end := l.Start, l.End
	if l.Outdated || end == 0 {
		start, end = l.origStart, l.origEnd
	}
	return fmt.Sprintf("GHDIFF_GOTO=%s:%s:%d:%d", l.Path, l.Side, start, end)
}

func (l CodeLoc) label() string {
	start, end := l.Start, l.End
	if l.Outdated || end == 0 {
		start, end = l.origStart, l.origEnd
	}
	s := fmt.Sprintf("%s#L%d", l.Path, end)
	if start > 0 && start != end {
		s = fmt.Sprintf("%s#L%d-L%d", l.Path, start, end)
	}
	if l.Outdated {
		s += " (outdated)"
	}
	return s
}

// reviewLocs are the code locations of the PR's review threads, in the order
// the Activity tab reads them.
func (m *Model) reviewLocs() []CodeLoc {
	if m.pr == nil || !m.pr.Data.IsEnriched {
		return nil
	}
	var locs []CodeLoc
	for _, t := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		side := t.DiffSide
		if side == "" {
			side = "RIGHT"
		}
		loc := CodeLoc{Path: t.Path, Side: side, Start: t.StartLine, End: t.Line,
			Outdated:  t.IsOutdated || t.Line == 0,
			origStart: t.OriginalStartLine, origEnd: t.OriginalLine}
		if loc.Start == 0 {
			loc.Start = loc.End
		}
		if loc.origStart == 0 {
			loc.origStart = loc.origEnd
		}
		if len(t.Comments.Nodes) > 0 {
			loc.hunk = t.Comments.Nodes[0].DiffHunk
		}
		locs = append(locs, loc)
	}
	return locs
}

const codeLocZonePrefix = "review-loc-"

func codeLocZone(thread, comment int) string {
	return codeLocZonePrefix + strconv.Itoa(thread) + "-" + strconv.Itoa(comment)
}

// CodeLocAt returns the review location under the mouse on the Activity tab.
func (m *Model) CodeLocAt(msg tea.MouseMsg) (CodeLoc, bool) {
	if zone.DefaultManager == nil || m.carousel.SelectedItem() != tabs[1] {
		return CodeLoc{}, false
	}
	locs := m.reviewLocs()
	for ti, t := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		for ci := range t.Comments.Nodes {
			if zone.Get(codeLocZone(ti, ci)).InBounds(msg) {
				return locs[ti], true
			}
		}
	}
	return CodeLoc{}, false
}

// renderCodeLoc is the "path#L…" line, with the code under it for the first
// comment of a thread, as one clickable zone.
func (m *Model) renderCodeLoc(loc CodeLoc, thread, comment, width int) string {
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	link := faint.Underline(true).Render(loc.label())
	out := link
	if comment == 0 {
		if code := m.renderHunk(loc, width); code != "" {
			out = lipgloss.JoinVertical(lipgloss.Left, link, code)
		}
	}
	if zone.DefaultManager == nil { // not set up (tests)
		return out
	}
	return zone.Mark(codeLocZone(thread, comment), out)
}

type hunkLine struct {
	kind     byte // ' ', '+' or '-'
	old, new int
	text     string
}

func parseHunk(hunk string) []hunkLine {
	var lines []hunkLine
	old, new := 0, 0
	for _, l := range strings.Split(hunk, "\n") {
		if strings.HasPrefix(l, "@@") {
			var a, c int
			// @@ -a[,b] +c[,d] @@
			fields := strings.Fields(l)
			if len(fields) >= 3 {
				a, _ = strconv.Atoi(strings.SplitN(strings.TrimPrefix(fields[1], "-"), ",", 2)[0])
				c, _ = strconv.Atoi(strings.SplitN(strings.TrimPrefix(fields[2], "+"), ",", 2)[0])
			}
			old, new = a, c
			continue
		}
		if l == "" || l[0] == '\\' {
			continue
		}
		hl := hunkLine{kind: l[0], text: strings.ReplaceAll(l[1:], "\t", "    ")}
		switch hl.kind {
		case '+':
			hl.new = new
			new++
		case '-':
			hl.old = old
			old++
		default:
			hl.kind = ' '
			hl.old, hl.new = old, new
			old++
			new++
		}
		lines = append(lines, hl)
	}
	return lines
}

// maxHunkLines caps the code shown under a comment.
const maxHunkLines = 12

// renderHunk shows the commented lines, with up to three lines before them.
func (m *Model) renderHunk(loc CodeLoc, width int) string {
	lines := parseHunk(loc.hunk)
	if len(lines) == 0 {
		return ""
	}
	on := func(l hunkLine) int {
		if loc.Side == "LEFT" {
			if l.kind == '+' {
				return 0
			}
			return l.old
		}
		if l.kind == '-' {
			return 0
		}
		return l.new
	}
	first, last := -1, -1
	for i, l := range lines {
		if n := on(l); n >= loc.origStart && n <= loc.origEnd && n > 0 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 { // not found: the hunk ends at the commented line
		first, last = len(lines)-1, len(lines)-1
	}
	// The hunk is from when the comment was made; while the thread is current
	// its lines have only moved, so number them as the label does.
	shift := 0
	if !loc.Outdated && loc.End > 0 {
		shift = loc.End - loc.origEnd
	}
	from := max(0, first-3)
	to := min(last, from+maxHunkLines-1)

	numW := len(strconv.Itoa(max(lines[to].old, lines[to].new) + max(shift, 0)))
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	add := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText)
	del := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)
	mark := lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Render("▌")
	rendered := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		l := lines[i]
		n := l.new
		if l.kind == '-' || (l.kind == ' ' && loc.Side == "LEFT") {
			n = l.old
		}
		if on(l) > 0 {
			n += shift
		}
		gutter := " "
		if i >= first && i <= last {
			gutter = mark
		}
		text := string(l.kind) + " " + l.text
		switch l.kind {
		case '+':
			text = add.Render(text)
		case '-':
			text = del.Render(text)
		}
		line := gutter + faint.Render(fmt.Sprintf("%*d ", numW, n)) + text
		rendered = append(rendered, ansi.Truncate(line, width, constants.Ellipsis))
	}
	if last > to {
		rendered = append(rendered, faint.Render(fmt.Sprintf("  … %d more lines", last-to)))
	}
	return strings.Join(rendered, "\n")
}
