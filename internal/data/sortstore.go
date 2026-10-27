package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"charm.land/log/v2"
)

// The column each section was last sorted by (a header click), kept in
// $XDG_STATE_HOME/gh-dash/sorts.json so it survives restarts.

type SectionSort struct {
	Key  string `json:"key"`
	Desc bool   `json:"desc"`
}

var (
	sortsMu     sync.Mutex
	sorts       map[string]SectionSort
	sortsLoaded bool
)

func sortsPath() string {
	p, err := getStateFilePath("sorts.json")
	if err != nil {
		log.Error("Failed to get state file path for sorts", "err", err)
		return ""
	}
	return p
}

func loadSorts() {
	if sortsLoaded {
		return
	}
	sortsLoaded, sorts = true, map[string]SectionSort{}
	if b, err := os.ReadFile(sortsPath()); err == nil {
		if err := json.Unmarshal(b, &sorts); err != nil {
			log.Error("Failed to read sorts", "err", err)
		}
	}
}

// GetSectionSort is the sort saved for section, if any.
func GetSectionSort(section string) (SectionSort, bool) {
	sortsMu.Lock()
	defer sortsMu.Unlock()
	loadSorts()
	s, ok := sorts[section]
	return s, ok
}

// SetSectionSort saves the sort for section.
func SetSectionSort(section string, s SectionSort) {
	sortsMu.Lock()
	defer sortsMu.Unlock()
	loadSorts()
	sorts[section] = s
	p := sortsPath()
	if p == "" {
		return
	}
	b, err := json.MarshalIndent(sorts, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(p), 0o755)
	}
	if err == nil {
		tmp := p + ".tmp"
		if err = os.WriteFile(tmp, b, 0o644); err == nil {
			err = os.Rename(tmp, p)
		}
	}
	if err != nil {
		log.Error("Failed to save sorts", "err", err)
	}
}
