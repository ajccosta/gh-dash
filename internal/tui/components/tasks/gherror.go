package tasks

import (
	"errors"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ghError turns a failed gh run into an error that says what gh printed on
// stderr (e.g. "Can not approve your own pull request"), instead of just
// "exit status 1". prefix, if set, names the action ("Approve #4 failed").
func ghError(err error, stderr string, prefix string) error {
	if err == nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(ansi.Strip(stderr), "\n") {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			lines = append(lines, l)
		}
	}
	msg := strings.Join(lines, " ")
	if msg == "" {
		msg = err.Error()
	}
	if prefix != "" {
		msg = prefix + ": " + msg
	}
	return errors.New(msg)
}
