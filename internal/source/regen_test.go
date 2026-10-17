package source

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/watch"
)

// update rewrites the fixtures of the grid and of the lists from the adapter:
//
//	TABLEAUX_DIR=…/tableaux go test ./internal/source -run 'Test(Grid|List)Fixtures' -update
var update = flag.Bool("update", false, "rewrite the fixtures of the grid and of the lists from the adapter")

// fixtureCommit is the commit of the umbrella's plan that the fixtures draw,
// the commit docs/mockups/tableau.md draws.
const fixtureCommit = "3cdae527d47d37fdff55bc302c76c017cf91c606"

// gridFixtures are the fixtures of the grid that a request of the umbrella's
// plan at 3cdae52 answers, by the request source.File names them for.
// tableau-empty.json, a project with no task, answers none.
var gridFixtures = []Request{
	{View: "tableau", Window: 1, Level: "provenance"},
	{View: "tableau", Window: 0, Level: "detail"},
	{View: "tableau", Window: 1, Historical: true, Level: "detail"},
	{View: "tableau", Window: 1, Person: "nbyoung@nbyoung.com", Level: "detail"},
	{View: "context", Window: 1, Person: "nbyoung@nbyoung.com", Level: "detail"},
}

// listFixtures are the fixtures of the four list views, in
// internal/view/list/testdata and again in internal/ui/detail/testdata, by
// the request of the umbrella's plan at 3cdae52 that answers each: the file
// source.File names for it, under dir. The queue asks for the person who
// holds the most work at that commit, and empty/queue.json for an email
// outside the project. Two files answer no request and stay as drawn:
// kinds/queue.json, one item of each kind, since no queue tablo derives from
// the umbrella or the corpus holds a review item; and empty/history-5fe3.json,
// a history with no event, since every task at 3cdae52 has a task file.
var listFixtures = []struct {
	dir string
	r   ListRequest
}{
	{"", ListRequest{View: "task", Task: "e9c6", Level: "detail"}},
	{"", ListRequest{View: "task", Task: "e9c6", Level: "provenance"}},
	{"", ListRequest{View: "task", Task: "5fe3", Level: "detail"}},
	{"", ListRequest{View: "task", Task: "5fe3", Level: "provenance"}},
	{"", ListRequest{View: "task", Task: "595e", Level: "detail"}},
	{"", ListRequest{View: "task", Task: "595e", Level: "provenance"}},
	{"", ListRequest{View: "queue", Level: "detail", Viewer: "noreply@anthropic.com"}},
	{"empty", ListRequest{View: "queue", Level: "glance", Viewer: "nobody@example.org"}},
	{"", ListRequest{View: "blockage", Task: "437e", Level: "detail"}},
	{"", ListRequest{View: "blockage", Task: "437e", Level: "provenance"}},
	{"", ListRequest{View: "blockage", Task: "e9c6", Level: "detail"}},
	{"", ListRequest{View: "history", Task: "e9c6", Level: "detail"}},
	{"", ListRequest{View: "history", Task: "e9c6", Level: "provenance"}},
	{"", ListRequest{View: "history", Task: "595e", Level: "detail"}},
	{"", ListRequest{View: "history", Task: "5fe3", Level: "glance"}},
}

// fixtureSource is the adapter on a clone of the umbrella on main at
// fixtureCommit, as the mockups read it, with that commit's snapshot stored.
func fixtureSource(t *testing.T) *Tablo {
	t.Helper()
	dir := corpustest.Umbrella(t, fixtureCommit)
	src, err := NewTablo(t.Context(), dir, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := src.Load(t.Context(), watch.State{Commit: fixtureCommit})
	if err != nil {
		t.Fatal(err)
	}
	src.Store(snap)
	return src
}

// matches compares the file at path with the adapter's envelope b, or with
// -update writes b in its place.
func matches(t *testing.T, path string, b []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got, had any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &had); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, had) || !bytes.Equal(b, want) {
		t.Errorf("%s differs from the adapter's envelope; -update rewrites it", path)
	}
}

// TestGridFixtures compares each fixture of the grid that a request of the
// umbrella's plan at 3cdae52 answers with the adapter's envelope for that
// request, on a clone of the umbrella on main at that commit, as the mockups
// read it. With -update it writes the adapter's envelope in its place. It
// skips without TABLEAUX_DIR.
func TestGridFixtures(t *testing.T) {
	src := fixtureSource(t)
	for _, r := range gridFixtures {
		name := File{}.Name(r)
		t.Run(name, func(t *testing.T) {
			v, p := tableauOf(r)
			b, _, err := src.view(t.Context(), v, p)
			if err != nil {
				t.Fatal(err)
			}
			matches(t, filepath.Join("..", "ui", "grid", "testdata", name), b)
		})
	}
}

// TestListFixtures does for the list fixtures what TestGridFixtures does for
// the grid's, in both directories that hold them.
func TestListFixtures(t *testing.T) {
	src := fixtureSource(t)
	for _, f := range listFixtures {
		name := filepath.Join(f.dir, File{}.ListName(f.r))
		t.Run(name, func(t *testing.T) {
			r := f.r
			b, _, err := src.view(t.Context(), tablo.View(r.View), tablo.Params{
				Viewer: r.Viewer, Role: tablo.Role(r.Role), Level: tablo.Level(r.Level), Task: r.Task,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{"../view/list/testdata", "../ui/detail/testdata"} {
				matches(t, filepath.Join(filepath.FromSlash(dir), name), b)
			}
		})
	}
}
