package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	f := File{Columns: map[string]Choice{"function": Hide, "design": Hide, "undefined": Show}}
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version || len(got.Columns) != 3 || got.Columns["undefined"] != Show || got.Columns["design"] != Hide {
		t.Errorf("Load = %+v", got)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"version\": 1,\n  \"columns\": {\n    \"design\": \"hide\",\n    \"function\": \"hide\",\n    \"undefined\": \"show\"\n  }\n}\n"
	if string(b) != want {
		t.Errorf("file =\n%s\nwant\n%s", b, want)
	}
}

func TestMissingFileMeansNoChoice(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || len(got.Columns) != 0 {
		t.Errorf("Load = %+v, %v", got, err)
	}
	got, err = Load("")
	if err != nil || len(got.Columns) != 0 {
		t.Errorf("Load(\"\") = %+v, %v", got, err)
	}
}

func TestRefusals(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"malformed":  "{not json",
		"version 2":  `{"version": 2, "columns": {}}`,
		"no version": `{"columns": {}}`,
		"bad choice": `{"version": 1, "columns": {"design": "maybe"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Error("Load: no error")
			}
		})
	}
}

func TestSaveModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix modes")
	}
	dir := filepath.Join(t.TempDir(), "tablotui")
	path := filepath.Join(dir, "settings.json")
	if err := Save(path, File{}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{dir: 0o700, path: 0o600} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", p, got, want)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want the settings file alone", len(entries))
	}
}

func TestSaveUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only directory stops no write here")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := Save(filepath.Join(dir, "settings.json"), File{}); err == nil {
		t.Error("Save: no error in a read-only directory")
	}
}

func TestSaveKeepsNoTemporaryFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	// A directory at the target path makes the rename fail.
	path := filepath.Join(dir, "settings.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, File{}); err == nil {
		t.Fatal("Save: no error over a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries after a failed save, want 1", len(entries))
	}
}

func TestKeyOfAnotherProjectSurvivesALoadAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, File{Columns: map[string]Choice{"performance": Hide}}); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Columns["design"] = Hide
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Columns["performance"] != Hide || got.Columns["design"] != Hide {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestDefaultPath(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("the configuration directory does not follow XDG_CONFIG_HOME here")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "tablotui", "settings.json"); got != want {
		t.Errorf("DefaultPath = %s, want %s", got, want)
	}
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err = DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(xdg, "tablotui", "settings.json"); got != want {
		t.Errorf("DefaultPath = %s, want %s", got, want)
	}
}
