package detail

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view/list"
)

// tabloGolden is one of tablo's golden files of the views the panes draw,
// copied unchanged into testdata/tablo/; the README there names the commit.
// The file holds the data of one envelope, and params are the parameters of
// tablo's golden table for it, without the level, which the name ends in.
type tabloGolden struct {
	command, file string
	params        map[string]any
}

var tabloGoldens = []tabloGolden{
	{"task", "task-3c5d.detail.json", map[string]any{"task": "3c5d", "person": nil}},
	{"task", "task-c07d.provenance.json", map[string]any{"task": "c07d", "person": "ben@example.org"}},
	{"queue", "queue-ada.glance.json", map[string]any{"task": "a1c0", "person": "ada@example.org", "brief": nil}},
	{"queue", "queue-dan.glance.json", map[string]any{"task": "a1c0", "person": "dan@example.org", "brief": nil}},
	{"blockage", "blockage.detail.json", map[string]any{"task": "a1c0", "person": nil}},
}

// level is the level the file's name ends in.
func (g tabloGolden) level() string {
	name := strings.TrimSuffix(g.file, ".json")
	return name[strings.LastIndex(name, ".")+1:]
}

// envelope wraps the file's data as tablo's schema test wraps its golden
// data: the source main with no commit and an undetermined trunk, no
// project, an observer, and the parameters with the role and the level.
func (g tabloGolden) envelope(t *testing.T) []byte {
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
	return env
}

// golden serves one golden file for its view, whatever the request asks.
type goldenSource struct {
	t *testing.T
	g tabloGolden
}

func (s goldenSource) decode(v list.View) error {
	return list.Decode(bytes.NewReader(s.g.envelope(s.t)), v)
}

func (s goldenSource) Task(context.Context, source.ListRequest) (v list.Task, err error) {
	return v, s.decode(&v)
}

func (s goldenSource) Queue(context.Context, source.ListRequest) (v list.Queue, err error) {
	return v, s.decode(&v)
}

func (s goldenSource) Blockage(context.Context, source.ListRequest) (v list.Blockage, err error) {
	return v, s.decode(&v)
}

func (s goldenSource) History(context.Context, source.ListRequest) (v list.History, err error) {
	return v, s.decode(&v)
}

// mentions lists the ids a pane must name for the data of a golden file.
func (s goldenSource) mentions(t *testing.T) []string {
	t.Helper()
	var ids []string
	switch s.g.command {
	case "task":
		v, err := s.Task(context.Background(), source.ListRequest{})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.Task.ID)
		for _, r := range v.Requires {
			ids = append(ids, r.Task.ID)
		}
	case "queue":
		v, err := s.Queue(context.Background(), source.ListRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range v.Items {
			ids = append(ids, it.Task.ID)
		}
	case "blockage":
		v, err := s.Blockage(context.Background(), source.ListRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range v.Causes {
			ids = append(ids, c.Task.ID)
		}
	case "history":
		v, err := s.History(context.Background(), source.ListRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range v.Lines {
			ids = append(ids, l.Task.ID)
		}
	}
	return ids
}

// TestTabloGoldens decodes each of tablo's golden files of the four views and
// draws its pane at the level the data holds: the document at the widths of
// the golden documents, every row within its width, the title at that level,
// the same bytes twice, and every task the data names; and the view at the
// sizes of the pane's own fit test, h lines of w cells under both measures.
func TestTabloGoldens(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range tabloGoldens {
		seen[g.file] = true
		t.Run(g.file, func(t *testing.T) {
			src := goldenSource{t, g}
			sel := "a1c0"
			if task, ok := g.params["task"].(string); ok {
				sel = task
			}
			draw := func(w, h int) *ph {
				return pane(t, g.command, "testdata", w, h).withSource(src).open().sel(sel, "")
			}
			for _, w := range []int{78, 38} {
				x := draw(w+2, 30)
				doc := x.doc(w)
				if again := draw(w+2, 30).doc(w); again != doc {
					t.Errorf("width %d: two drawings differ", w)
				}
				title := strings.SplitN(doc, "\n", 2)[0]
				if !strings.HasSuffix(title, " · "+g.level()) || strings.Contains(doc, "error:") {
					t.Errorf("width %d: title %q\n%s", w, title, doc)
				}
				for i, l := range strings.Split(strings.TrimSuffix(doc, "\n"), "\n")[1:] {
					if got := ansi.StringWidth(l); got > w {
						t.Errorf("width %d: row %d is %d cells: %q", w, i, got, l)
					}
				}
				for _, id := range src.mentions(t) {
					if !strings.Contains(doc, id) {
						t.Errorf("width %d: the document never names %s\n%s", w, id, doc)
					}
				}
			}
			for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
				x := draw(80, 20)
				x.c.Measure = ui.Measure{Method: method}
				for _, keys := range []string{"", "f", "end", "f end"} {
					x.keys(keys)
					for w := 1; w <= 90; w += 3 {
						for _, h := range []int{1, 2, 5, 12, 40} {
							lines := strings.Split(x.p.View(w, h, x.c), "\n")
							if len(lines) != h {
								t.Fatalf("%dx%d: %d lines", w, h, len(lines))
							}
							for i, l := range lines {
								if got := x.c.Measure.Width(ansi.Strip(l)); got != w {
									t.Fatalf("%dx%d keys %q: line %d is %d cells wide: %q", w, h, keys, i, got, l)
								}
							}
						}
					}
				}
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
