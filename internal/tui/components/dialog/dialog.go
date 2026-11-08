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
// typed instead: Options are its suggestions, the one picked (Sel, among those
// that match Text) shown grey after Text, and tab completes it. Left empty,
// its value is the first option.
type Field struct {
	Name    string // the key of its value in what Submit gets
	Title   string
	Options []string
	Sel     int
	Input   bool
	Text    string
	fresh   bool // Text is the starting value: typing replaces it
}

// suggestions are the options an input field's Text could become: those that
// start with it, then those that contain it (ignoring case). A fresh field
// suggests every other option.
func (f Field) suggestions() []int {
	var starts, contains []int
	t := strings.ToLower(f.Text)
	for i, o := range f.Options {
		lo := strings.ToLower(o)
		switch {
		case o == f.Text || o == "":
		case f.fresh || strings.HasPrefix(lo, t):
			starts = append(starts, i)
		case strings.Contains(lo, t):
			contains = append(contains, i)
		}
	}
	return append(starts, contains...)
}

// suggestion is the picked suggestion's option index, or -1.
func (f Field) suggestion() int {
	s := f.suggestions()
	if len(s) == 0 {
		return -1
	}
	return s[min(f.Sel, len(s)-1)]
}

// ghost is what tab adds to Text: the rest of the picked suggestion, when it
// starts with Text.
func (f Field) ghost() string {
	i := f.suggestion()
	if i < 0 || f.fresh || !strings.HasPrefix(strings.ToLower(f.Options[i]), strings.ToLower(f.Text)) {
		return ""
	}
	return f.Options[i][len(f.Text):]
}

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
			f.Sel, f.fresh = 0, f.Text != ""
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
						f.Text, f.Sel, f.fresh = f.Options[oi], 0, false
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
// text, backspace, tab completing, ←→ picking another suggestion.
func (m *Model) typeIn(k tea.KeyMsg) bool {
	f := &m.spec.Fields[m.row]
	key := k.Key()
	switch s := k.String(); {
	case s == "tab":
		g := f.ghost()
		if i := f.suggestion(); g == "" && (i < 0 || f.fresh || f.Text == "") {
			return false
		} else if g == "" {
			g = f.Options[i] // one that contains Text: becomes it
			f.Text = ""
		}
		f.Text, f.Sel, f.fresh = f.Text+g, 0, false
	case s == "backspace":
		if f.fresh {
			f.Text = ""
		} else if r := []rune(f.Text); len(r) > 0 {
			f.Text = string(r[:len(r)-1])
		}
		f.Sel, f.fresh = 0, false
	case s == "ctrl+u":
		f.Text, f.Sel, f.fresh = "", 0, false
	case s == "left" || s == "right":
		if n := len(f.suggestions()); n > 0 {
			d := 1
			if s == "left" {
				d = n - 1
			}
			f.Sel = (min(f.Sel, n-1) + d) % n
		}
	case key.Text != "" && key.Mod&^tea.ModShift == 0:
		if f.fresh {
			f.Text = ""
		}
		f.Text, f.Sel, f.fresh = f.Text+key.Text, 0, false
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
			lines = append(lines, label+"  "+m.inputView(fi, max(20, min(70, m.ctx.ScreenWidth-titleW-16))))
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
		lines = append(lines, faint.Render("type · tab complete · ←→ suggestion · ↑↓ move · enter confirm · esc cancel"))
	default:
		lines = append(lines, faint.Render("↑↓ move · ←→ choose · enter confirm · esc cancel"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.PrimaryBorder).
		Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// inputView is an input field: its text, the rest of the picked suggestion in
// grey, then the other suggestions, fainter, as many as fit in width.
func (m Model) inputView(fi int, width int) string {
	t := m.ctx.Theme
	f := m.spec.Fields[fi]
	faint := lipgloss.NewStyle().Foreground(t.FaintText)
	text := lipgloss.NewStyle().Foreground(t.PrimaryText)
	if f.fresh {
		text = text.Background(t.SelectedBackground) // typing replaces it
	}
	ghost := f.ghost()
	shown := ghost != "" // the picked suggestion is already
	s := " " + text.Render(f.Text)
	if fi == m.row {
		cur := " "
		if ghost != "" {
			cur, ghost = ghost[:1], ghost[1:]
		}
		s += lipgloss.NewStyle().Reverse(true).Render(cur)
	}
	s += faint.Render(ghost)
	used := lipgloss.Width(s)
	picked := f.suggestion()
	for _, oi := range f.suggestions() {
		o := f.Options[oi]
		if shown && oi == picked {
			continue
		}
		if used+lipgloss.Width(o)+3 > width {
			s += faint.Render("  …")
			break
		}
		st := faint.Faint(true)
		if oi == picked {
			st = faint.Underline(true)
		}
		s += "  " + zone.Mark(optionZone(fi, oi), st.Render(o))
		used += lipgloss.Width(o) + 2
	}
	return s
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
func visibleOptions(f Field, width int) (int, int) {
	w := func(i int) int { return lipgloss.Width(f.Options[i]) + 3 } // " o " and the gap
	lo, hi, used := f.Sel, f.Sel, w(f.Sel)+4                         // room for ‹ and ›
	for grew := true; grew; {
		grew = false
		if hi+1 < len(f.Options) && used+w(hi+1) <= width {
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
