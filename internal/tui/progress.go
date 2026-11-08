package tui

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/shell"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// A keybinding form with progress: true runs its command in the background,
// the dialog drawing a bar the command moves by printing lines:
//
//	progress PCT TEXT   the bar at PCT (0-100), TEXT the current step
//	run COMMAND         run COMMAND in the terminal once this one is done
//
// Its other lines, and stderr, show in the dialog if it fails.

// startProgressMsg starts command under the dialog's title and text.
type startProgressMsg struct {
	title   string
	text    []string
	command string
}

// progressRun is a running command; its messages come through lines.
type progressRun struct {
	lines chan tea.Msg
}

type progressLineMsg struct {
	run  *progressRun
	pct  int // -1: keep the bar where it is
	step string
}

type progressDoneMsg struct {
	run    *progressRun
	err    error
	output []string // the last lines it wrote that were not progress
	then   string   // its "run" line
}

// progressTail is how many of a failed command's lines the dialog shows.
const progressTail = 8

func (r *progressRun) next() tea.Cmd {
	return func() tea.Msg { return <-r.lines }
}

// updateProgress handles the progress messages; ok when msg was one.
func (m *Model) updateProgress(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case startProgressMsg:
		return m.startProgress(msg), true
	case progressLineMsg:
		if msg.run == m.progressRun {
			m.dialog.SetProgress(msg.pct, msg.step)
		}
		return msg.run.next(), true
	case progressDoneMsg:
		if msg.run != m.progressRun {
			return nil, true // cancelled
		}
		m.progressRun = nil
		if msg.err != nil {
			m.dialog.FailProgress(append(msg.output, msg.err.Error()))
			return nil, true
		}
		m.dialog.Close()
		if msg.then != "" {
			return m.executeCustomCommand(msg.then), true
		}
		return nil, true
	}
	return nil, false
}

func (m *Model) startProgress(p startProgressMsg) tea.Cmd {
	c := shell.Command(p.command)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return func() tea.Msg { return constants.ErrMsg{Err: err} }
	}
	var stderr bytes.Buffer
	c.Stderr = &stderr
	inGroup(c)
	if err := c.Start(); err != nil {
		return func() tea.Msg { return constants.ErrMsg{Err: err} }
	}
	run := &progressRun{lines: make(chan tea.Msg)}
	m.progressRun = run
	m.dialog.OpenProgress(p.title, p.text, func() {
		m.progressRun = nil
		killGroup(c)
	})
	go func() {
		var output []string
		then := ""
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Text()
			if rest, ok := strings.CutPrefix(line, "progress "); ok {
				num, step, _ := strings.Cut(rest, " ")
				pct, err := strconv.Atoi(num)
				if err != nil {
					pct = -1
				}
				run.lines <- progressLineMsg{run: run, pct: pct, step: step}
			} else if rest, ok := strings.CutPrefix(line, "run "); ok {
				then = rest
			} else if line != "" {
				output = append(output, line)
			}
		}
		err := c.Wait()
		output = append(output, strings.Split(strings.TrimSpace(stderr.String()), "\n")...)
		output = output[max(0, len(output)-progressTail):]
		run.lines <- progressDoneMsg{run: run, err: err, output: output, then: then}
	}()
	return run.next()
}
