package prssection

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
)

// GitHub works out whether a PR can be merged in the background, and says
// UNKNOWN until it has: for a while after the PR or its base changed, as for
// the other PRs on a branch a PR was just merged into; asking is what makes
// it start. Such a PR keeps what GitHub said last, so it doesn't show as free
// of conflicts in the meantime, and GitHub is asked again until it knows:
// after mergeableEvery, then twice as long each time, mergeableTries times at
// most (about 2.5 minutes). A PR is asked about by one poll at a time, however
// often the list is fetched meanwhile.

var (
	mergeableMu     sync.Mutex
	knownMergeable  = map[string]string{} // by mergeableKey
	askingMergeable = map[string]bool{}   // the PRs a poll is asking about, by mergeableKey
	mergeableEvery  = 5 * time.Second
)

const mergeableTries = 5

func mergeableKey(repo string, number int) string { return repo + "#" + strconv.Itoa(number) }

// mergeableMsg is what GitHub said, once it knew or gave up.
type mergeableMsg struct {
	repo   string
	number int
	value  string
}

// keepMergeable gives the PRs GitHub doesn't know about yet what it said last,
// remembers what it knows of the others, and asks again about the unknown ones.
func (m *Model) keepMergeable(prs []prrow.Data) tea.Cmd {
	mergeableMu.Lock()
	defer mergeableMu.Unlock()
	var cmds []tea.Cmd
	for i := range prs {
		p := prs[i].Primary
		if p == nil || p.State != "OPEN" {
			continue
		}
		key := mergeableKey(p.Repository.NameWithOwner, p.Number)
		if p.Mergeable != "UNKNOWN" {
			knownMergeable[key] = p.Mergeable
			continue
		}
		if last, ok := knownMergeable[key]; ok {
			p.Mergeable = last
		}
		if askingMergeable[key] {
			continue
		}
		askingMergeable[key] = true
		cmds = append(cmds, m.MakeSectionCmd(askMergeable(p.Repository.NameWithOwner, p.Number)))
	}
	return tea.Batch(cmds...)
}

// askMergeable asks GitHub until it knows whether the PR can be merged.
func askMergeable(repo string, number int) tea.Cmd {
	return func() tea.Msg {
		value := "UNKNOWN"
		for try := range mergeableTries {
			time.Sleep(mergeableEvery << try)
			out, err := exec.Command("gh", "pr", "view", strconv.Itoa(number), "-R", repo,
				"--json", "mergeable", "--jq", ".mergeable").Output()
			if err != nil {
				break
			}
			if value = strings.TrimSpace(string(out)); value != "UNKNOWN" {
				break
			}
		}
		return mergeableMsg{repo, number, value}
	}
}

// setMergeable takes what GitHub said about a PR.
func (m *Model) setMergeable(msg mergeableMsg) {
	key := mergeableKey(msg.repo, msg.number)
	mergeableMu.Lock()
	delete(askingMergeable, key)
	if msg.value == "UNKNOWN" || msg.value == "" {
		mergeableMu.Unlock()
		return
	}
	knownMergeable[key] = msg.value
	mergeableMu.Unlock()
	for i, pr := range m.Prs {
		if p := pr.Primary; p != nil && p.Number == msg.number && p.Repository.NameWithOwner == msg.repo {
			m.Prs[i].Primary.Mergeable = msg.value
		}
	}
	m.Table.SetRows(m.BuildRows())
}
