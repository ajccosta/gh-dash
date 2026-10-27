package tui

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/cli/go-gh/v2/pkg/browser"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// Clicking a link (defaults.mouse) opens it. Links are the terminal
// hyperlinks (OSC 8) in the view, as glamour writes them for markdown, so
// any component's links work with nothing from the component.

var osc8 = regexp.MustCompile("\x1b\\]8;[^;\x07\x1b]*;([^\x07\x1b]*)(?:\x07|\x1b\\\\)")

// markLinks puts each hyperlink's text in a zone "link-N" (one per line it
// spans). bubblezone counts a hyperlink's URL as text, so the hyperlinks are
// swapped for placeholders, which the returned func puts back after the scan.
func (m *Model) markLinks(s string) (string, func(string) string) {
	if !strings.Contains(s, "\x1b]8;") {
		m.setLinks(nil)
		return s, func(s string) string { return s }
	}
	var links []string
	var restore []string
	var url string // the hyperlink open at this point, if any
	marker := func() string {
		start, _, _ := strings.Cut(zone.Mark(fmt.Sprintf("link-%d", len(links)-1), "x"), "x")
		return start // a zone's start and end markers are the same
	}
	open := func(b *strings.Builder) {
		links = append(links, url)
		b.WriteString(marker())
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		var b strings.Builder
		if url != "" {
			open(&b)
		}
		// glamour splits a link's text at "_" and the like into hyperlinks
		// that follow each other: a closed link's zone is only ended once
		// what follows isn't the same link again
		closed := false
		last := 0
		for _, mt := range osc8.FindAllStringSubmatchIndex(line, -1) {
			gap, next := line[last:mt[0]], line[mt[2]:mt[3]]
			ph := fmt.Sprintf("\x1b[%d;7731Y", len(restore)/2)
			restore = append(restore, ph, line[mt[0]:mt[1]])
			switch {
			case url != "": // ends or replaces the open link
				b.WriteString(gap + ph)
				if next == "" {
					closed = true
				} else {
					b.WriteString(marker())
				}
			case closed && next == links[len(links)-1] && ansi.StringWidth(gap) == 0:
				b.WriteString(gap + ph) // the same link again: one zone
				closed = false
				url = next
				last = mt[1]
				continue
			case closed:
				b.WriteString(marker() + gap + ph)
				closed = false
			default:
				b.WriteString(gap + ph)
			}
			url = next
			if url != "" {
				open(&b)
			}
			last = mt[1]
		}
		if closed {
			b.WriteString(marker())
		}
		b.WriteString(line[last:])
		if url != "" {
			b.WriteString(marker())
		}
		lines[i] = b.String()
	}
	m.setLinks(links)
	r := strings.NewReplacer(restore...)
	return strings.Join(lines, "\n"), r.Replace
}

// setLinks records the view's links, and forgets the zones of links that
// are gone, so they aren't hovered or clicked.
func (m *Model) setLinks(links []string) {
	for i := len(links); i < len(m.links); i++ {
		zone.Clear(fmt.Sprintf("link-%d", i))
	}
	m.links = links
}

// linkAt is the URL of the link under the mouse, if any.
func (m *Model) linkAt(msg tea.MouseMsg) string {
	for i, url := range m.links {
		if zone.Get(fmt.Sprintf("link-%d", i)).InBounds(msg) {
			return url
		}
	}
	return ""
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		// Discard the launcher's output, so it can't corrupt the display.
		if err := browser.New("", io.Discard, io.Discard).Browse(url); err != nil {
			return constants.ErrMsg{Err: err}
		}
		return nil
	}
}
