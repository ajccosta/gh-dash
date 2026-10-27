package markdown

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderHangsListItems(t *testing.T) {
	in := "- one two three four five six seven eight nine ten eleven twelve\n" +
		"  - nested one two three four five six seven eight nine ten\n" +
		"1. first one two three four five six seven eight nine ten"
	out, err := GetMarkdownRenderer(30, nil).Render(in)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range strings.Split(ansi.Strip(out), "\n") {
		if l = strings.TrimRight(l, " "); l != "" {
			got = append(got, l)
		}
	}
	want := []string{
		"• one two three four five six",
		"  seven eight nine ten eleven",
		"  twelve",
		"  • nested one two three four",
		"    five six seven eight nine",
		"    ten",
		"1. first one two three four",
		"   five six seven eight nine",
		"   ten",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
