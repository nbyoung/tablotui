// Package settings keeps the per-viewer column choices of the tableau grid in
// one JSON file. A choice hides or shows the column of one gate, whatever the
// window says; the choices hold for every project the viewer opens.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Version is the one file version this package reads and writes.
const Version = 1

// Choice is what a viewer chose for one gate column.
type Choice string

// The two choices. A gate with no entry follows the window.
const (
	Hide Choice = "hide"
	Show Choice = "show"
)

// File is the content of the settings file. Columns maps a gate key to its choice.
type File struct {
	Version int               `json:"version"`
	Columns map[string]Choice `json:"columns"`
}

// DefaultPath returns the settings file of the viewer: tablotui/settings.json
// under os.UserConfigDir.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tablotui", "settings.json"), nil
}

// Load reads the settings at path. A missing file, or an empty path, gives no
// choice and no error. A file that does not parse, states another version or
// holds a choice other than hide and show gives an error.
func Load(path string) (File, error) {
	empty := File{Version: Version, Columns: map[string]Choice{}}
	if path == "" {
		return empty, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return empty, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version != Version {
		return empty, fmt.Errorf("%s: version %d, want %d", path, f.Version, Version)
	}
	if f.Columns == nil {
		f.Columns = map[string]Choice{}
	}
	for k, c := range f.Columns {
		if c != Hide && c != Show {
			return empty, fmt.Errorf("%s: column %q has the choice %q, want hide or show", path, k, c)
		}
	}
	return f, nil
}

// Save writes f to path with sorted keys, a two-space indent and a final
// newline. It writes a temporary file in the same directory and renames it, so
// a reader never sees half a file. The directory takes mode 0700 and the file
// mode 0600. An empty path writes nothing.
func Save(path string, f File) error {
	if path == "" {
		return nil
	}
	f.Version = Version
	if f.Columns == nil {
		f.Columns = map[string]Choice{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(b); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
