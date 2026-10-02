package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// settings is the file that keeps the hidden gate columns between sessions.
type settings struct {
	Hidden []string `json:"hidden_gates"`
}

// LoadHidden reads the hidden gates from path. A missing file means none.
func LoadHidden(path string) (map[string]bool, error) {
	hidden := map[string]bool{}
	if path == "" {
		return hidden, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return hidden, nil
	}
	if err != nil {
		return hidden, err
	}
	var s settings
	if err := json.Unmarshal(b, &s); err != nil {
		return hidden, err
	}
	for _, g := range s.Hidden {
		hidden[g] = true
	}
	return hidden, nil
}

// SaveHidden writes the hidden gates to path, sorted, through a temporary file.
func SaveHidden(path string, hidden map[string]bool) error {
	if path == "" {
		return nil
	}
	s := settings{Hidden: []string{}}
	for g, h := range hidden {
		if h {
			s.Hidden = append(s.Hidden, g)
		}
	}
	sort.Strings(s.Hidden)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
