package grid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/source"
)

// goldenCase is one row of the table under T1 of design 679b.
type goldenCase struct {
	name     string
	fixture  source.Request
	w, h     int
	keys     string
	settings string
	after    map[string]settings.Choice // the file after the keys; nil leaves it unchecked
}

var (
	reqGlobal     = source.Request{View: "tableau", Window: 1, Level: "detail"}
	reqWindow0    = source.Request{View: "tableau", Window: 0, Level: "detail"}
	reqHistorical = source.Request{View: "tableau", Window: 1, Historical: true, Level: "detail"}
	reqPerson     = source.Request{View: "tableau", Window: 1, Person: "nbyoung@nbyoung.com", Level: "detail"}
)

const fourHidden = `{"version":1,"columns":{"function":"hide","design":"hide","release":"hide","performance":"hide"}}`

var goldens = []goldenCase{
	{name: "glance-100x14", fixture: reqGlobal, w: 100, h: 14},
	{name: "detail-100x24", fixture: reqGlobal, w: 100, h: 24, keys: "E d down down"},
	{name: "provenance-100x30", fixture: reqGlobal, w: 100, h: 30, keys: "E p down down right right right right right right"},
	{name: "rollup-100x20", fixture: reqGlobal, w: 100, h: 20, keys: "p right right"},
	{name: "snapshot-100x20", fixture: reqGlobal, w: 100, h: 20, keys: "down down down down p right right"},
	{name: "window0-80x16", fixture: reqWindow0, w: 80, h: 16, keys: "down enter right d"},
	{name: "hidden-80x16", fixture: reqGlobal, w: 80, h: 16, keys: "down enter right right right right x right x",
		after: map[string]settings.Choice{"design": settings.Hide, "function": settings.Hide}},
	{name: "settings-80x16", fixture: reqGlobal, w: 80, h: 16, keys: "down enter right s", settings: fourHidden,
		after: map[string]settings.Choice{"function": settings.Hide, "design": settings.Hide, "release": settings.Hide,
			"performance": settings.Hide, "undefined": settings.Show}},
	{name: "narrow-48x12", fixture: reqGlobal, w: 48, h: 12, keys: "down enter right right right right right right right right right"},
	{name: "historical-100x16", fixture: reqHistorical, w: 100, h: 16, keys: "down enter"},
	{name: "person-100x16", fixture: reqPerson, w: 100, h: 16, keys: "down enter"},
	{name: "empty-60x10", fixture: source.Request{View: "tableau", Window: 1, Level: "detail"}, w: 60, h: 10},
}

// T1: the screens.
func TestGoldenScreens(t *testing.T) {
	for _, c := range goldens {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := fixtures()
			if c.name == "empty-60x10" {
				// The empty project has a fixture of its own under the name the request asks for.
				copyFile(t, filepath.Join("testdata", "tableau-empty.json"), filepath.Join(dir, "tableau.json"))
				src = source.File{Dir: dir}
			}
			path := settingsFile(t, c.settings)
			start := GlanceStart()
			start.Request = c.fixture
			r := newRig(t, src, path, start, c.w, c.h).keys(c.keys)
			want := normalise(readFile(t, filepath.Join("testdata", c.name+".txt")))
			if got := r.screen(); got != want {
				t.Errorf("screen differs from %s.txt\n--- got\n%s\n--- want\n%s", c.name, got, want)
			}
			if c.after != nil {
				got := loadSettings(t, path)
				if len(got) != len(c.after) {
					t.Errorf("settings after = %v, want %v", got, c.after)
				}
				for k, v := range c.after {
					if got[k] != v {
						t.Errorf("settings after = %v, want %v", got, c.after)
					}
				}
			}
		})
	}
}

func normalise(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		t.Fatal(err)
	}
}
