package common

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoveredCmd_KeepsTheAltScreenAround(t *testing.T) {
	var out bytes.Buffer
	c := &coveredCmd{Cmd: exec.Command("printf", "PAGER"), note: "Opening"}
	c.SetStdin(nil)
	c.SetStdout(&out)
	c.SetStderr(&out)
	require.NoError(t, c.Run())
	require.Equal(t, coverScreen+"\n  \x1b[2mOpening\x1b[0m"+"PAGER"+coverScreen, out.String())
}
