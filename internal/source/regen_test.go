package source

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/watch"
)

// update rewrites the grid's fixtures from the adapter:
//
//	TABLEAUX_DIR=…/tableaux go test ./internal/source -run TestGridFixtures -update
var update = flag.Bool("update", false, "rewrite the fixtures of internal/ui/grid/testdata from the adapter")

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

// TestGridFixtures compares each fixture of the grid that a request of the
// umbrella's plan at 3cdae52 answers with the adapter's envelope for that
// request, on a clone of the umbrella on main at that commit, as the mockups
// read it. With -update it writes the adapter's envelope in its place. It
// skips without TABLEAUX_DIR.
func TestGridFixtures(t *testing.T) {
	const commit = "3cdae527d47d37fdff55bc302c76c017cf91c606"
	dir := corpustest.Umbrella(t, commit)
	src, err := NewTablo(t.Context(), dir, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := src.Load(t.Context(), watch.State{Commit: commit})
	if err != nil {
		t.Fatal(err)
	}
	src.Store(snap)
	for _, r := range gridFixtures {
		name := File{}.Name(r)
		t.Run(name, func(t *testing.T) {
			v, p := tableauOf(r)
			b, _, err := src.view(t.Context(), v, p)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("..", "ui", "grid", "testdata", name)
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
				t.Errorf("%s differs from the adapter's envelope; -update rewrites it", name)
			}
		})
	}
}
