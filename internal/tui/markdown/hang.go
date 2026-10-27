package markdown

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// itemStart matches a rendered list item's marker, after its indent.
var itemStart = regexp.MustCompile(`^( *)(• |\d+\. |\[[ ✓]\] )`)

// hangListItems rewraps each list item so its wrapped lines start under its
// text instead of under its marker.
func hangListItems(out string, width int) string {
	lines := strings.Split(out, "\n")
	var res []string
	for i := 0; i < len(lines); {
		plain := strings.TrimRight(ansi.Strip(lines[i]), " ")
		m := itemStart.FindStringSubmatch(plain)
		if m == nil {
			res = append(res, lines[i])
			i++
			continue
		}
		indent := len(m[1])
		hang := indent + ansi.StringWidth(m[2])
		// the item's wrapped lines: same indent, not blank, not an item
		j := i + 1
		for ; j < len(lines); j++ {
			p := strings.TrimRight(ansi.Strip(lines[j]), " ")
			if p == "" || itemStart.MatchString(p) || len(p)-len(strings.TrimLeft(p, " ")) != indent {
				break
			}
		}
		if j == i+1 {
			res = append(res, lines[i])
			i++
			continue
		}
		first := trimRightSpace(lines[i])
		text := ansi.TruncateLeft(first, hang, "")
		for _, l := range lines[i+1 : j] {
			text += " " + trimLeftSpace(trimRightSpace(l))
		}
		wrapped := strings.Split(ansi.Wordwrap(text, max(width-hang, 10), ""), "\n")
		for k, w := range wrapped {
			if k == 0 {
				w = ansi.Truncate(first, hang, "") + w
			} else {
				w = strings.Repeat(" ", hang) + trimLeftSpace(w)
			}
			res = append(res, w)
		}
		i = j
	}
	return strings.Join(res, "\n")
}

// trimRightSpace drops the trailing spaces glamour pads lines with, keeping
// any escape codes among them.
func trimRightSpace(s string) string {
	for {
		t := strings.TrimRight(s, " ")
		if strings.HasSuffix(t, "\x1b[m") || strings.HasSuffix(t, "\x1b[0m") {
			i := strings.LastIndex(t, "\x1b[")
			if strings.TrimRight(t[:i], " ") != t[:i] {
				s = strings.TrimRight(t[:i], " ") + t[i:]
				continue
			}
		}
		return t
	}
}

// trimLeftSpace drops leading spaces, keeping escape codes before them.
func trimLeftSpace(s string) string {
	var esc strings.Builder
	for {
		s = strings.TrimLeft(s, " ")
		if loc := leadingEsc.FindStringIndex(s); loc != nil {
			esc.WriteString(s[:loc[1]])
			s = s[loc[1]:]
			continue
		}
		return esc.String() + s
	}
}

var leadingEsc = regexp.MustCompile(`^\x1b\[[0-9;]*m`)
