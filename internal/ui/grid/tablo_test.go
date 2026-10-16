package grid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/view"
)

// tabloGolden is one of tablo's golden files of the tableaux, copied
// unchanged into testdata/tablo/; the README there names the commit. The file
// holds the data of one envelope, and params are the parameters of tablo's
// golden table for it, without the level, which the name ends in.
type tabloGolden struct {
	command, file string
	params        map[string]any
}

var tabloGoldens = []tabloGolden{
	{"tableau", "tableau.detail.json", map[string]any{"task": nil, "person": nil, "window": 1, "columns": nil, "historical": false}},
	{"context", "context-ben.detail.json", map[string]any{"task": nil, "person": "ben@example.org", "window": 1, "columns": nil, "historical": false}},
	{"context", "context-4e2b.glance.json", map[string]any{"task": "4e2b", "person": nil, "window": 1, "columns": nil, "historical": false}},
}

// level is the level the file's name ends in.
func (g tabloGolden) level() string {
	name := strings.TrimSuffix(g.file, ".json")
	return name[strings.LastIndex(name, ".")+1:]
}

// decode wraps the file's data as tablo's schema test wraps its golden data,
// the source main with no commit and an undetermined trunk, no project, an
// observer, and the parameters with the role and the level, and decodes it.
func (g tabloGolden) decode(t *testing.T) view.Tableau {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "tablo", g.file))
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]any{"role": "observer", "level": g.level()}
	for k, v := range g.params {
		params[k] = v
	}
	env, err := json.Marshal(map[string]any{
		"schema":  "tablo/1",
		"command": g.command,
		"source": map[string]any{
			"ref": "main", "worktree": false, "commit": nil, "date": nil, "from": nil,
			"trunk": map[string]any{"name": nil, "resolved": "undetermined"}, "on_trunk": false,
		},
		"project":     nil,
		"viewer":      map[string]any{"email": nil, "roles": []string{"observer"}},
		"params":      params,
		"data":        json.RawMessage(raw),
		"diagnostics": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	tb, err := view.Decode(bytes.NewReader(env))
	if err != nil {
		t.Fatalf("%s: %v", g.file, err)
	}
	return tb
}

// TestTabloGoldens decodes each of tablo's golden files of the tableaux and
// draws the grid at each level the data holds, at the sizes of the golden
// screens and the floor, under both measures: every screen has its height in
// lines, each exactly its width in cells; the context line names the ref;
// every row draws; and the strip names each row with its status.
func TestTabloGoldens(t *testing.T) {
	sizes := [][2]int{{100, 14}, {100, 16}, {100, 20}, {100, 24}, {100, 30}, {80, 16}, {60, 10}, {48, 12}, {40, 8}}
	scripts := map[string][]string{
		"glance": {"", "E", "E right right right x left s"},
		"detail": {"", "E", "E right right right x left s", "E d down down", "E d end right right right right", "E p down right right right right right"},
	}
	seen := map[string]bool{}
	for _, g := range tabloGoldens {
		seen[g.file] = true
		tb := g.decode(t)
		if tb.Command != g.command || tb.Params.Level != g.level() || len(tb.Rows) == 0 {
			t.Fatalf("%s: command %q, level %q, %d rows", g.file, tb.Command, tb.Params.Level, len(tb.Rows))
		}
		src := &recorder{fn: func(source.Request) (view.Tableau, error) { return tb, nil }}
		for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
			for si, script := range scripts[g.level()] {
				for _, s := range sizes {
					t.Run(fmt.Sprintf("%s/%d/script%d/%dx%d", g.file, method, si, s[0], s[1]), func(t *testing.T) {
						r := newRig(t, src, "", GlanceStart(), s[0], s[1])
						if method == ansi.GraphemeWidth {
							r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
						}
						r.keys(script)
						lines := r.content()
						if len(lines) != s[1] {
							t.Fatalf("%d lines", len(lines))
						}
						for i, l := range lines {
							if got := method.StringWidth(ansi.Strip(l)); got != s[0] {
								t.Fatalf("line %d is %d cells: %q", i, got, ansi.Strip(l))
							}
						}
					})
				}
			}
		}
		t.Run(g.file+"/content", func(t *testing.T) {
			r := newRig(t, src, "", GlanceStart(), 140, 60).keys("E d")
			if got := r.line(0); got != "main" {
				t.Errorf("context line = %q", got)
			}
			for i, row := range tb.Rows {
				if !strings.Contains(r.screen(), row.ID+" ") {
					t.Errorf("row %s does not draw", row.ID)
				}
				if got, want := r.line(ruleAt(r)+1), row.ID+" "+row.Title+" · "; !strings.HasPrefix(got, want) {
					t.Errorf("strip of row %d = %q, want it to start %q", i, got, want)
				}
				if got := r.line(ruleAt(r) + 1); !strings.Contains(got, " "+row.Status.Gate+" ") || !strings.Contains(got, row.Status.State) {
					t.Errorf("strip of row %s = %q, want its status", row.ID, got)
				}
				r.keys("down")
			}
		})
	}
	files, err := filepath.Glob(filepath.Join("testdata", "tablo", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !seen[filepath.Base(f)] {
			t.Errorf("%s is in no case", f)
		}
	}
}
