package data

import (
	"encoding/json"
	"os"
	"path/filepath"

	"charm.land/log/v2"
)

// LoadState reads $XDG_STATE_HOME/gh-dash/<name> into v (left alone if the
// file is missing or unreadable).
func LoadState(name string, v any) {
	p, err := getStateFilePath(name)
	if err != nil {
		return
	}
	if b, err := os.ReadFile(p); err == nil {
		if err := json.Unmarshal(b, v); err != nil {
			log.Error("Failed to read "+name, "err", err)
		}
	}
}

// SaveState writes v to $XDG_STATE_HOME/gh-dash/<name>.
func SaveState(name string, v any) {
	p, err := getStateFilePath(name)
	if err != nil {
		return
	}
	b, err := json.MarshalIndent(v, "", "  ")
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
		log.Error("Failed to save "+name, "err", err)
	}
}
