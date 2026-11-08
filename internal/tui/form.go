package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/dialog"
)

// withForm runs a custom keybinding's command at once or, when the keybinding
// has a form, once its dialog is confirmed: the command then reads the
// choices as {{.Form.<name>}}. Each field starts at the option chosen last
// time. row is what the command is about, shown under the title.
func (m *Model) withForm(kb config.Keybinding, row any, run func() tea.Cmd) tea.Cmd {
	if kb.Form == nil {
		return run()
	}
	title := kb.Form.Title
	if title == "" {
		title = kb.Name
	}
	fields := make([]dialog.Field, 0, len(kb.Form.Fields))
	for _, f := range kb.Form.Fields {
		opts := append([]string{}, f.Options...)
		if f.OptionsCommand != "" {
			opts = append(opts, commandLines(f.OptionsCommand)...)
		}
		if len(opts) == 0 {
			opts = []string{""}
		}
		label := f.Title
		if label == "" {
			label = f.Name
		}
		fields = append(fields, dialog.Field{Name: f.Name, Title: label, Options: opts})
	}
	var text []string
	if r, ok := row.(interface {
		GetNumber() int
		GetTitle() string
	}); ok {
		text = append(text, fmt.Sprintf("#%d %s", r.GetNumber(), r.GetTitle()))
	}
	m.dialog.Open(dialog.Spec{
		Title:    title,
		Text:     text,
		Fields:   fields,
		Buttons:  []string{"Run", "Cancel"},
		Remember: kb.Key + " " + title,
		Submit: func(values map[string]string, _ string) tea.Cmd {
			m.formValues = values
			if kb.Form.Progress {
				m.formProgress = &startProgressMsg{title: title, text: text}
			}
			defer func() { m.formValues, m.formProgress = nil, nil }()
			return run()
		},
	})
	return nil
}

// commandLines is what a form field's optionsCommand prints, a line each.
func commandLines(command string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sh", "-c", command).Output()
	if err != nil {
		log.Error("form options command failed", "command", command, "err", err)
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
