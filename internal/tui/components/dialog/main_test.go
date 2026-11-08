package dialog

import (
	"os"
	"testing"
)

// TestMain keeps the tests' state out of
// the user's $XDG_STATE_HOME.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gh-dash-test-state")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
