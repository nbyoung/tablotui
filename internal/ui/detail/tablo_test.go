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
	{"history", "history.glance.json", map[string]any{"task": nil, "person": nil}},
	{"history", "history-4e2b.detail.json", map[string]any{"task": "4e2b", "person": nil}},
	{"history", "history-c07d.provenance.json", map[string]any{"task": "c07d", "person": nil}},
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
			if s := l.Taken(); s != nil {
				ids = append(ids, s.Task)
			} else if l.Task.ID != v.Subject.ID {
				ids = append(ids, l.Task.ID)
			}
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

// tabloDoc draws the document of one of tablo's golden files at width w.
func tabloDoc(t *testing.T, file string, w int) string {
	t.Helper()
	for _, g := range tabloGoldens {
		if g.file == file {
			sel, _ := g.params["task"].(string)
			x := pane(t, g.command, "testdata", w+2, 30).withSource(goldenSource{t, g}).open().sel(sel, "")
			return x.doc(w)
		}
	}
	t.Fatalf("no golden %s", file)
	return ""
}

// TestTabloSubEventTitle: a line taken from a subproject, beneath its pin or
// in its own place, and an event of the task pane name the subproject's task
// by id and title, as tablo's golden data gives the title (8ed1 decision 8).
func TestTabloSubEventTitle(t *testing.T) {
	for file, want := range map[string]string{
		"history-4e2b.detail.json":     "f1a0 Weather node image",
		"history-c07d.provenance.json": "f1a0 Weather node image",
		"task-c07d.provenance.json":    "in the subproject firmware, task f1a0 Weather node image",
	} {
		if doc := tabloDoc(t, file, 400); !strings.Contains(doc, want) {
			t.Errorf("%s lacks %q:\n%s", file, want, doc)
		}
	}
}

// TestTabloQueueCauses: the queue at glance words a reason from its wait's
// note and an authorisation from its why, as tablo's glance gives them
// (886d decision 15).
func TestTabloQueueCauses(t *testing.T) {
	doc := tabloDoc(t, "queue-ada.glance.json", 400)
	for _, want := range []string{
		"9f31 Sensor board; blocked: Barometer ICs on 14-week backorder ·",
		"3c5d Dashboard; proposed; the deciding commit is not by an authority ·",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the glance lacks %q:\n%s", want, doc)
		}
	}
}

// TestQueueCauseReadsTheItemAlone: a reason reads its wait's note and never
// the item's status, and an authorisation reads its why and never its
// authorisation.
func TestQueueCauseReadsTheItemAlone(t *testing.T) {
	w := words{}
	waiting := list.QueueItem{
		Kind:   "waiting",
		Causes: []list.Wait{{Kind: "reason", Reason: "blocked", Note: "the wait's note"}},
		Status: &list.Status{Note: "the status's note"},
	}
	if got, want := w.queueCause(waiting), "blocked: the wait's note"; got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}
	waiting.Causes[0].Note = ""
	if got, want := w.queueCause(waiting), "blocked"; got != want {
		t.Errorf("reason with no note = %q, want %q", got, want)
	}
	owed := list.QueueItem{Kind: "authorisation", Why: "no_commit", Authorisation: &list.Authorisation{Why: "off_trunk"}}
	if got, want := w.queueCause(owed), "proposed; no commit on the trunk decides it"; got != want {
		t.Errorf("authorisation = %q, want %q", got, want)
	}
}

// TestTabloOwnCommit: the task pane at provenance draws, in the section
// "Status commit", the status commit and then the commit that decides a
// snapshot task's own file under "deciding commit", as tabloio's task view
// does (e3ed decision 18). A task with no own commit draws the status commit
// alone.
func TestTabloOwnCommit(t *testing.T) {
	doc := tabloDoc(t, "task-c07d.provenance.json", 78)
	want := "\nStatus commit\n  commit            cf5b169 Record the node image design\n"
	own := "\n  deciding commit   97e4231 Pin the firmware at its design\n  date              2026-09-26\n"
	i, j := strings.Index(doc, want), strings.Index(doc, own)
	if i < 0 || j < i || strings.Contains(doc[i+1:j], "\n\n") {
		t.Errorf("the section lacks the status commit, then the own commit:\n%s", doc)
	}
	x := pane(t, "task", "testdata", 80, 30).open().sel("e9c6", "").keys("p")
	if doc := x.doc(78); !strings.Contains(doc, "\nStatus commit\n") || strings.Contains(doc, "deciding commit") {
		t.Errorf("a task with no own commit draws one, or no status commit:\n%s", doc)
	}
}
