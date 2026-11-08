// Package dialog is a small form drawn in the middle of the screen: a title,
// a few lines of text, rows of choices and a row of buttons. Merging a PR and
// the custom keybindings that declare a form use it. It can also show a
// command's progress as a bar (OpenProgress).
package dialog

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// Field is one row of choices; Sel is the chosen option. An Input field is
// one that can also be typed: its Options are shown as a choice row, ←→ make
// Text the previous or next of them, and typing filters them, the rest of the
// first match shown grey after Text for tab to complete. Left empty, its
// value is the first option.
type Field struct {
	Name    string // the key of its value in what Submit gets
	Title   string
	Options []string
	Sel     int
	Input   bool
	Text    string
	query   string // what was typed last: the options shown match it
	fresh   bool   // Text was chosen, not typed: typing replaces it
}

// suggestions are the options an input field shows: with nothing typed all
// of them, else those that start with what was typed, then those that
// contain it (ignoring case).
func (f Field) suggestions() []int {
	var starts, contains []int
	q := strings.ToLower(f.query)
	for i, o := range f.Options {
		lo := strings.ToLower(o)
		switch {
		case o == "":
		case strings.HasPrefix(lo, q):
			starts = append(starts, i)
		case strings.Contains(lo, q):
			contains = append(contains, i)
		}
	}
	return append(starts, contains...)
}

// ghost is what tab adds to Text while typing: the rest of the first
// suggestion, when it starts with Text.
func (f Field) ghost() string {
	s := f.suggestions()
	if f.fresh || f.Text == "" || len(s) == 0 ||
		!strings.HasPrefix(strings.ToLower(f.Options[s[0]]), strings.ToLower(f.Text)) {
		return ""
	}
	return f.Options[s[0]][len(f.Text):]
}

// choose makes Text option oi, as ←→ and clicks do; typing then replaces it.
func (f *Field) choose(oi int) { f.Text, f.fresh = f.Options[oi], true }

// typed makes Text what the user typed, filtering the options by it.
func (f *Field) typed(text string) { f.Text, f.query, f.fresh = text, text, false }

func (f Field) value() string {
	if !f.Input {
		return f.Options[f.Sel]
	}
	if t := strings.TrimSpace(f.Text); t != "" || len(f.Options) == 0 {
		return t
	}
	return f.Options[0] // left empty
}

// Spec describes a dialog. Submit gets each field's chosen option by name
// and the pressed button; the last button cancels and calls nothing. A dialog
// with a Remember key starts each field at the option chosen last time it was
// submitted, kept in $XDG_STATE_HOME/gh-dash/forms.json under that key.
type Spec struct {
	Title    string
	Text     []string
	Fields   []Field
	Buttons  []string
	Submit   func(values map[string]string, button string) tea.Cmd
	Remember string
}

type Model struct {
	ctx  *context.ProgramContext
	spec *Spec
	row  int // the focused field; len(Fields) is the buttons row
	btn  int
	prog *progress
}

// progress is a running command's: how far, its current step, the question
// it waits on, and once it failed, what it said. Its one button cancels it,
// or closes once it failed.
type progress struct {
	pct    int
	step   string
	ask    *question
	failed []string
	cancel func()
}

// question is a running command's, one option per line; answer gets the
// picked option's index.
type question struct {
	text    string
	options []string
	sel     int
	answer  func(int)
}

func New(ctx *context.ProgramContext) Model { return Model{ctx: ctx} }

func (m *Model) Open(spec Spec) {
	var last map[string]string
	if spec.Remember != "" {
		last = data.GetFormChoices(spec.Remember)
	}
	spec.Fields = slices.Clone(spec.Fields)
	for i := range spec.Fields {
		f := &spec.Fields[i]
		switch {
		case f.Input && last[f.Name] != "":
			f.Text = last[f.Name]
		case f.Input && f.Text == "" && len(f.Options) > 0:
			f.Text = f.Options[f.Sel]
		}
		if f.Input {
			f.query, f.fresh = "", f.Text != ""
		} else if o := slices.Index(f.Options, last[f.Name]); o >= 0 {
			f.Sel = o
		}
	}
	m.spec, m.row, m.btn, m.prog = &spec, 0, 0, nil
}

func (m *Model) Close()      { m.spec, m.prog = nil, nil }
func (m Model) IsOpen() bool { return m.spec != nil }

// OpenProgress shows a bar under title and text until Close; cancel runs if
// the user stops it first.
func (m *Model) OpenProgress(title string, text []string, cancel func()) {
	m.Open(Spec{Title: title, Text: text, Buttons: []string{"Cancel"}})
	m.prog = &progress{cancel: cancel}
}

func (m Model) InProgress() bool { return m.prog != nil }

// SetProgress moves the bar to pct (kept when negative) and shows step.
func (m *Model) SetProgress(pct int, step string) {
	if m.prog == nil {
		return
	}
	if pct >= 0 {
		m.prog.pct = min(100, pct)
	}
	m.prog.step = step
}

// AskProgress shows text and options, one per line, until the user picks
// one, which answer gets (its index); esc cancels the command instead.
func (m *Model) AskProgress(text string, options []string, answer func(int)) {
	if m.prog == nil || len(options) == 0 {
		return
	}
	m.prog.ask = &question{text: text, options: options, answer: answer}
}

// pick answers the question with option i and goes back to the bar.
func (m *Model) pick(i int) {
	q := m.prog.ask
	m.prog.ask = nil
	q.answer(i)
}

// FailProgress shows what the command said instead of the bar.
func (m *Model) FailProgress(lines []string) {
	if m.prog == nil {
		return
	}
	m.prog.failed, m.prog.cancel, m.prog.ask = append([]string{"Failed:"}, lines...), nil, nil
	m.spec.Buttons = []string{"Close"}
}

// stop cancels a running command, if it still runs, and closes the dialog.
func (m *Model) stop() {
	if m.prog.cancel != nil {
		m.prog.cancel()
	}
	m.Close()
}
func (m Model) Values() map[string]string {
	v := map[string]string{}
	for _, f := range m.spec.Fields {
		v[f.Name] = f.value()
	}
	return v
}

// press runs the button's action and closes the dialog.
func (m *Model) press(b int) tea.Cmd {
	spec, values := m.spec, m.Values()
	m.Close()
	if b == len(spec.Buttons)-1 || spec.Submit == nil {
		return nil
	}
	if spec.Remember != "" {
		data.SetFormChoices(spec.Remember, values)
	}
	return spec.Submit(values, spec.Buttons[b])
}

// Update takes every key and click while the dialog is open.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.spec == nil {
		return nil
	}
	if m.prog != nil {
		q := m.prog.ask
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch k := msg.String(); {
			case k == "esc" || k == "q" || k == "ctrl+c":
				m.stop()
			case q == nil && k == "enter":
				m.stop()
			case q == nil:
			case k == "enter":
				m.pick(q.sel)
			case k == "up" || k == "k" || k == "shift+tab":
				q.sel = (q.sel + len(q.options) - 1) % len(q.options)
			case k == "down" || k == "j" || k == "tab":
				q.sel = (q.sel + 1) % len(q.options)
			case len(k) == 1 && k[0] >= '1' && int(k[0]-'1') < len(q.options):
				m.pick(int(k[0] - '1'))
			}
		case tea.MouseClickMsg:
			x, y := m.offset()
			msg.X, msg.Y = msg.X-x, msg.Y-y
			if q != nil {
				for i := range q.options {
					if zone.Get(questionZone(i)).InBounds(msg) {
						m.pick(i)
						return nil
					}
				}
			}
			if zone.Get(buttonZone(0)).InBounds(msg) {
				m.stop()
			}
		}
		return nil
	}
	rows := len(m.spec.Fields) + 1
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.row < len(m.spec.Fields) && m.spec.Fields[m.row].Input && m.typeIn(msg) {
			return nil
		}
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			m.Close()
		case "up", "k", "shift+tab":
			m.row = (m.row + rows - 1) % rows
		case "down", "j", "tab":
			m.row = (m.row + 1) % rows
		case "left", "h":
			m.step(-1)
		case "right", "l":
			m.step(1)
		case "enter":
			if m.row < len(m.spec.Fields) {
				return m.press(0)
			}
			return m.press(m.btn)
		}
	case tea.MouseClickMsg:
		// The zones were scanned in the dialog's own box: click in its frame.
		x, y := m.offset()
		msg.X, msg.Y = msg.X-x, msg.Y-y
		for fi := range m.spec.Fields {
			f := &m.spec.Fields[fi]
			for oi := range f.Options {
				if zone.Get(optionZone(fi, oi)).InBounds(msg) {
					m.row = fi
					if f.Input {
						f.choose(oi)
					} else {
						f.Sel = oi
					}
				}
			}
		}
		for b := range m.spec.Buttons {
			if zone.Get(buttonZone(b)).InBounds(msg) {
				return m.press(b)
			}
		}
	}
	return nil
}

// typeIn edits the focused input field with key k, if it is one that does:
// ←→ choosing the previous or next option shown, text, backspace, tab
// completing.
func (m *Model) typeIn(k tea.KeyMsg) bool {
	f := &m.spec.Fields[m.row]
	key := k.Key()
	sugg := f.suggestions()
	cur := slices.IndexFunc(sugg, func(oi int) bool { return f.Options[oi] == f.Text })
	switch s := k.String(); {
	case s == "left" || s == "right":
		if n := len(sugg); n > 0 {
			switch {
			case cur >= 0 && s == "left":
				cur = (cur + n - 1) % n
			case cur >= 0:
				cur = (cur + 1) % n
			case s == "left":
				cur = n - 1
			default:
				cur = 0 // the one shown grey
			}
			f.choose(sugg[cur])
		}
	case s == "tab":
		if cur >= 0 || len(sugg) == 0 {
			return false // nothing to complete: the next row
		}
		f.Text, f.fresh = f.Options[sugg[0]], false
	case s == "backspace":
		if r := []rune(f.Text); len(r) > 0 {
			f.typed(string(r[:len(r)-1]))
		}
	case s == "ctrl+u":
		f.typed("")
	case key.Text != "" && key.Mod&^tea.ModShift == 0:
		if f.fresh {
			f.Text = ""
		}
		f.typed(f.Text + key.Text)
	default:
		return false
	}
	return true
}

func (m *Model) step(d int) {
	if m.row < len(m.spec.Fields) {
		f := &m.spec.Fields[m.row]
		f.Sel = (f.Sel + len(f.Options) + d) % len(f.Options)
		return
	}
	n := len(m.spec.Buttons)
	m.btn = (m.btn + n + d) % n
}

func optionZone(field, option int) string { return fmt.Sprintf("dialog-opt-%d-%d", field, option) }
func buttonZone(b int) string             { return fmt.Sprintf("dialog-btn-%d", b) }
func questionZone(i int) string           { return fmt.Sprintf("dialog-ask-%d", i) }

// View is the dialog's box; Layer places it in the middle of the screen.
func (m Model) View() string {
	if m.spec == nil {
		return ""
	}
	t := m.ctx.Theme
	faint := lipgloss.NewStyle().Foreground(t.FaintText)
	chosen := lipgloss.NewStyle().Foreground(t.PrimaryText).Background(t.SelectedBackground).Bold(true)
	focused := lipgloss.NewStyle().Foreground(t.PrimaryText).Bold(true)

	titleW := 0
	for _, f := range m.spec.Fields {
		titleW = max(titleW, lipgloss.Width(f.Title))
	}

	lines := []string{lipgloss.NewStyle().Bold(true).Foreground(t.PrimaryText).Render(m.spec.Title)}
	for _, s := range m.spec.Text {
		lines = append(lines, faint.Render(s))
	}
	lines = append(lines, "")
	if m.prog != nil {
		lines = append(lines, m.progressLines()...)
	}
	for fi, f := range m.spec.Fields {
		label := fmt.Sprintf("%-*s", titleW, f.Title)
		if fi == m.row {
			label = focused.Render("› " + label)
		} else {
			label = faint.Render("  " + label)
		}
		if f.Input {
			lines = append(lines, label+"  "+m.inputView(fi, max(20, min(70, m.ctx.ScreenWidth-titleW-16)), chosen))
			continue
		}
		lo, hi := visibleOptions(f, max(20, min(70, m.ctx.ScreenWidth-titleW-16)))
		opts := []string{}
		if lo > 0 {
			opts = append(opts, faint.Render("‹"))
		}
		for oi := lo; oi <= hi; oi++ {
			s := " " + f.Options[oi] + " "
			if oi == f.Sel {
				s = chosen.Render(s)
			} else {
				s = faint.Render(s)
			}
			opts = append(opts, zone.Mark(optionZone(fi, oi), s))
		}
		if hi < len(f.Options)-1 {
			opts = append(opts, faint.Render("›"))
		}
		lines = append(lines, label+"  "+strings.Join(opts, " "))
	}
	if len(m.spec.Fields) > 0 {
		lines = append(lines, "")
	}
	btns := make([]string, len(m.spec.Buttons))
	for b, name := range m.spec.Buttons {
		st := lipgloss.NewStyle().Padding(0, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.FaintBorder)
		if m.row == len(m.spec.Fields) && b == m.btn {
			st = st.BorderForeground(t.PrimaryText).Bold(true)
		} else if b == 0 && m.row < len(m.spec.Fields) {
			st = st.BorderForeground(t.SecondaryBorder) // what enter on a field does
		}
		btns[b] = zone.Mark(buttonZone(b), st.Render(name))
	}
	row := []string{}
	for b, btn := range btns {
		if b > 0 {
			row = append(row, " ")
		}
		row = append(row, btn)
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	switch {
	case m.prog != nil && m.prog.failed != nil:
		lines = append(lines, faint.Render("enter or esc close"))
	case m.prog != nil && m.prog.ask != nil:
		lines = append(lines, faint.Render("↑↓ move · enter or 1-9 choose · esc cancel"))
	case m.prog != nil:
		lines = append(lines, faint.Render("esc cancel"))
	case m.row < len(m.spec.Fields) && m.spec.Fields[m.row].Input:
		lines = append(lines, faint.Render("←→ choose · type to filter · tab complete · ↑↓ move · enter confirm · esc cancel"))
	default:
		lines = append(lines, faint.Render("↑↓ move · ←→ choose · enter confirm · esc cancel"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.PrimaryBorder).
		Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// inputView is an input field: its text and the rest of the first match in
// grey, then the options it shows as a choice row, the one Text is
// highlighted, as many as fit in width.
func (m Model) inputView(fi int, width int, chosen lipgloss.Style) string {
	t := m.ctx.Theme
	f := m.spec.Fields[fi]
	faint := lipgloss.NewStyle().Foreground(t.FaintText)
	ghost := f.ghost()
	completes := ghost != ""
	s := lipgloss.NewStyle().Foreground(t.PrimaryText).Render(f.Text)
	if fi == m.row {
		cur := " "
		if ghost != "" {
			cur, ghost = ghost[:1], ghost[1:]
		}
		s += lipgloss.NewStyle().Reverse(true).Render(cur)
	}
	s = faint.Render("[") + s + faint.Render(ghost+"]")
	sugg := f.suggestions()
	if len(sugg) == 0 {
		return s
	}
	opts := make([]string, len(sugg))
	sel := 0
	for i, oi := range sugg {
		opts[i] = f.Options[oi]
		if opts[i] == f.Text {
			sel = i
		}
	}
	lo, hi := window(opts, sel, width-lipgloss.Width(s)-2)
	row := []string{s, ""}
	if lo > 0 {
		row = append(row, faint.Render("‹"))
	}
	for i := lo; i <= hi; i++ {
		st := faint
		switch {
		case opts[i] == f.Text:
			st = chosen
		case i == 0 && completes:
			st = faint.Underline(true) // what tab completes
		}
		row = append(row, zone.Mark(optionZone(fi, sugg[i]), st.Render(" "+opts[i]+" ")))
	}
	if hi < len(opts)-1 {
		row = append(row, faint.Render("›"))
	}
	return strings.Join(row, " ")
}

// progressBarWidth is the bar's length in cells.
const progressBarWidth = 44

// progressLines is the step and the bar, or what a failed command said.
func (m Model) progressLines() []string {
	t := m.ctx.Theme
	faint := lipgloss.NewStyle().Foreground(t.FaintText)
	if m.prog.failed != nil {
		red := lipgloss.NewStyle().Foreground(t.ErrorText)
		var lines []string
		for _, l := range m.prog.failed {
			lines = append(lines, red.Render(ansi.Truncate(l, progressBarWidth+20, "…")))
		}
		return append(lines, "")
	}
	if q := m.prog.ask; q != nil {
		chosen := lipgloss.NewStyle().Foreground(t.PrimaryText).Background(t.SelectedBackground).Bold(true)
		lines := []string{lipgloss.NewStyle().Foreground(t.PrimaryText).Bold(true).Render(q.text)}
		for i, o := range q.options {
			s := fmt.Sprintf(" %d  %s ", i+1, ansi.Truncate(o, progressBarWidth+20, "…"))
			if i == q.sel {
				s = chosen.Render(s)
			} else {
				s = faint.Render(s)
			}
			lines = append(lines, zone.Mark(questionZone(i), s))
		}
		return append(lines, "")
	}
	done := progressBarWidth * m.prog.pct / 100
	bar := lipgloss.NewStyle().Foreground(t.SuccessText).Render(strings.Repeat("━", done)) +
		faint.Render(strings.Repeat("━", progressBarWidth-done)) +
		fmt.Sprintf(" %3d%%", m.prog.pct)
	return []string{faint.Render(ansi.Truncate(m.prog.step, progressBarWidth+5, "…")), bar, ""}
}

// visibleOptions is the run of options shown when they don't all fit in
// width: the chosen one and as many around it as fit, ‹ › marking the rest.
func visibleOptions(f Field, width int) (int, int) { return window(f.Options, f.Sel, width) }

// window is the run of opts around sel that fits in width.
func window(opts []string, sel int, width int) (int, int) {
	w := func(i int) int { return lipgloss.Width(opts[i]) + 3 } // " o " and the gap
	lo, hi, used := sel, sel, w(sel)+4                          // room for ‹ and ›
	for grew := true; grew; {
		grew = false
		if hi+1 < len(opts) && used+w(hi+1) <= width {
			hi++
			used += w(hi)
			grew = true
		}
		if lo > 0 && used+w(lo-1) <= width {
			lo--
			used += w(lo)
			grew = true
		}
	}
	return lo, hi
}

// offset is where the box's top left corner goes: the middle of the screen.
func (m Model) offset() (int, int) {
	v := m.View()
	return max(0, (m.ctx.ScreenWidth-lipgloss.Width(v))/2),
		max(0, (m.ctx.ScreenHeight-lipgloss.Height(v))/2)
}

// Layer is the dialog in the middle of the screen, above everything.
func (m Model) Layer() *lipgloss.Layer {
	x, y := m.offset()
	return lipgloss.NewLayer(zone.Scan(m.View())).X(x).Y(y).Z(10)
}
