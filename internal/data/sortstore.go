package data

import "sync"

// The column each section was last sorted by (a header click), kept in
// $XDG_STATE_HOME/gh-dash/sorts.json so it survives restarts.

type SectionSort struct {
	Key  string `json:"key"`
	Desc bool   `json:"desc"`
}

var (
	sortsMu sync.Mutex
	sorts   map[string]SectionSort
)

func loadSorts() {
	if sorts == nil {
		sorts = map[string]SectionSort{}
		LoadState("sorts.json", &sorts)
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
	SaveState("sorts.json", sorts)
}
