package common

import (
	"io"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// Escape codes: to the alternate screen, cleared, with no cursor.
const coverScreen = "\x1b[?1049h\x1b[2J\x1b[H\x1b[?25l"

// ExecCovered runs c in the terminal as tea.ExecProcess does, but keeps the
// alternate screen up around it: gh-dash leaves it to hand the terminal over,
// and the shell's screen would show while c starts (gh fetching the diff
// before the pager draws) and once it is done (until gh-dash draws again).
// note shows meanwhile.
func ExecCovered(c *exec.Cmd, note string, fn tea.ExecCallback) tea.Cmd {
	return tea.Exec(&coveredCmd{Cmd: c, note: note}, fn)
}

type coveredCmd struct {
	*exec.Cmd
	note string
	out  io.Writer
}

func (c *coveredCmd) Run() error {
	if c.out != nil {
		io.WriteString(c.out, coverScreen+"\n  \x1b[2m"+c.note+"\x1b[0m")
	}
	err := c.Cmd.Run()
	if c.out != nil {
		io.WriteString(c.out, coverScreen)
	}
	return err
}

func (c *coveredCmd) SetStdin(r io.Reader) {
	if c.Stdin == nil {
		c.Stdin = r
	}
}

func (c *coveredCmd) SetStdout(w io.Writer) {
	c.out = w
	if c.Stdout == nil {
		c.Stdout = w
	}
}

func (c *coveredCmd) SetStderr(w io.Writer) {
	if c.Stderr == nil {
		c.Stderr = w
	}
}
