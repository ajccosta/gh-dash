package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"
)

func TestLinks_ZonesSkipHyperlinkURLs(t *testing.T) {
	m := &Model{}
	zone.NewGlobal()
	zone.SetEnabled(true)
	link := func(url, text string) string { return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\" }
	view := "ab " + link("https://a.io/long/url", "one") + " " + link("https://b.io", "two") + "\n" +
		link("https://c.io", "wr\naps") + " " + link("https://d.io", "x_") + "\x1b[m" + link("https://d.io", "y")

	marked, restore := m.markLinks(view)
	require.Equal(t, view, restore(zone.Scan(marked)))
	require.Equal(t, []string{"https://a.io/long/url", "https://b.io", "https://c.io", "https://c.io", "https://d.io"}, m.links)

	scanZone(t, marked, "link-4")
	at := func(x, y int) string { return m.linkAt(tea.MouseClickMsg{X: x, Y: y}) }
	require.Equal(t, "", at(1, 0))
	require.Equal(t, "https://a.io/long/url", at(3, 0))
	require.Equal(t, "https://a.io/long/url", at(5, 0))
	require.Equal(t, "", at(6, 0))
	require.Equal(t, "https://b.io", at(7, 0)) // not shifted by the first URL
	require.Equal(t, "https://c.io", at(1, 1))
	require.Equal(t, "https://c.io", at(2, 2))
	require.Equal(t, "", at(3, 2))
	for x := 4; x <= 6; x++ { // a link glamour split at "_" is one zone
		require.Equal(t, "https://d.io", at(x, 2))
	}
	require.Equal(t, "", at(7, 2))
}
