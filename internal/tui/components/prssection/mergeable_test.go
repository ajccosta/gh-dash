package prssection

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

func mergeablePR(n int, mergeable string) prrow.Data {
	p := &data.PullRequestData{Number: n, State: "OPEN", Mergeable: mergeable}
	p.Repository.NameWithOwner = "o/mergeable"
	return prrow.Data{Primary: p}
}

func TestKeepMergeable_UnknownKeepsLastAndAsksAgain(t *testing.T) {
	bin := t.TempDir() // gh knows by the second time it's asked
	script := "#!/bin/sh\nf='" + filepath.Join(bin, "asked") + "'\n" +
		"[ -e \"$f\" ] && echo CONFLICTING || { touch \"$f\"; echo UNKNOWN; }\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	mergeableEvery = 0
	t.Cleanup(func() { mergeableEvery = 5 * time.Second })
	m := newTestModel("")

	require.Nil(t, m.keepMergeable([]prrow.Data{mergeablePR(19, "CONFLICTING")}))

	// After a merge into its base: GitHub is working it out again.
	prs := []prrow.Data{mergeablePR(19, "UNKNOWN"), mergeablePR(20, "UNKNOWN")}
	cmd := m.keepMergeable(prs)
	require.Equal(t, "CONFLICTING", prs[0].Primary.Mergeable, "what GitHub said last")
	require.Equal(t, "UNKNOWN", prs[1].Primary.Mergeable, "never known")
	require.NotNil(t, cmd)

	msg := askMergeable("o/mergeable", 19)().(mergeableMsg)
	require.Equal(t, "CONFLICTING", msg.value, "asked until GitHub knew")
}
