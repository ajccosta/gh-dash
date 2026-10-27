package prview

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

func TestParseHunk_NumbersBothSides(t *testing.T) {
	lines := parseHunk("@@ -10,4 +10,5 @@ func f() {\n a\n-b\n+c\n+d\n e")
	require.Len(t, lines, 5)
	require.Equal(t, hunkLine{kind: ' ', old: 10, new: 10, text: "a"}, lines[0])
	require.Equal(t, hunkLine{kind: '-', old: 11, text: "b"}, lines[1])
	require.Equal(t, hunkLine{kind: '+', new: 11, text: "c"}, lines[2])
	require.Equal(t, hunkLine{kind: '+', new: 12, text: "d"}, lines[3])
	require.Equal(t, hunkLine{kind: ' ', old: 12, new: 13, text: "e"}, lines[4])
}

func TestCodeLoc_LabelAndGoto(t *testing.T) {
	l := CodeLoc{Path: "a/b.go", Side: "RIGHT", Start: 12, End: 15, origStart: 10, origEnd: 13}
	require.Equal(t, "a/b.go#L12-L15", l.label())
	require.Equal(t, "GHDIFF_GOTO=a/b.go:RIGHT:12:15", l.GotoEnv())

	l = CodeLoc{Path: "a/b.go", Side: "RIGHT", Outdated: true, origStart: 7, origEnd: 7}
	require.Equal(t, "a/b.go#L7 (outdated)", l.label())
	require.Equal(t, "GHDIFF_GOTO=a/b.go:RIGHT:7:7", l.GotoEnv())
}

func TestCodeLoc_ClickableOnActivityTab(t *testing.T) {
	m := newTestModelWithWidth(t, &data.PullRequestData{State: "OPEN", Title: "t", Number: 1}, nil, nil, 100)
	threads := &m.pr.Data.Enriched.ReviewThreads
	var node = struct {
		Id                string
		IsOutdated        bool
		OriginalLine      int
		OriginalStartLine int
		StartLine         int
		Line              int
		DiffSide          string
		StartDiffSide     string
		Path              string
		Comments          data.ReviewComments `graphql:"comments(first: 20)"`
	}{Path: "a/b.go", Line: 12, OriginalLine: 12, DiffSide: "RIGHT"}
	node.Comments.Nodes = []data.ReviewComment{{Body: "hm", DiffHunk: "@@ -10,3 +10,3 @@\n a\n b\n+c"}}
	threads.Nodes = append(threads.Nodes, node)
	m.SelectTab(1)

	zone.NewGlobal()
	zone.SetEnabled(true)
	view := m.View()
	zone.Scan(view)
	var z *zone.ZoneInfo
	require.Eventually(t, func() bool {
		z = zone.Get(codeLocZone(0, 0))
		return !z.IsZero()
	}, time.Second, 5*time.Millisecond, "zone not found in:\n%s", view)
	loc, ok := m.CodeLocAt(tea.MouseClickMsg{X: z.StartX + 1, Y: z.StartY, Button: tea.MouseLeft})
	require.True(t, ok)
	require.Equal(t, "GHDIFF_GOTO=a/b.go:RIGHT:12:12", loc.GotoEnv())
}

func TestRenderHunk_NumbersCurrentLines(t *testing.T) {
	m := newTestModelWithWidth(t, &data.PullRequestData{State: "OPEN", Title: "t", Number: 1}, nil, nil, 100)
	loc := CodeLoc{Path: "a.go", Side: "RIGHT", Start: 20, End: 20, origStart: 18, origEnd: 18,
		hunk: "@@ -0,0 +17,2 @@\n+// doc\n+func F() {}"}
	out := ansi.Strip(m.renderHunk(loc, 80))
	require.Contains(t, out, "19 + // doc")
	require.Contains(t, out, "20 + func F() {}")
}
