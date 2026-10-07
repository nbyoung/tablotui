package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimal = `{"view":"tableau","project":"P","window":1,"gates":[],"rows":[]}`

func TestFileName(t *testing.T) {
	f := File{Dir: "x"}
	cases := []struct {
		r    Request
		want string
	}{
		{Request{View: "tableau", Window: 1}, "tableau.json"},
		{Request{View: "tableau", Window: 0}, "tableau-window0.json"},
		{Request{View: "tableau", Window: 2, Historical: true}, "tableau-window2-historical.json"},
		{Request{View: "tableau", Window: 1, Person: "ada@example.org"}, "tableau-person.json"},
		{Request{View: "context", Window: 1, Task: "c2ad", Level: "provenance"}, "context.json"},
		{Request{Window: 1}, "tableau.json"},
	}
	for _, c := range cases {
		if got := f.Name(c.r); got != c.want {
			t.Errorf("Name(%+v) = %s, want %s", c.r, got, c.want)
		}
	}
}

func TestFileTableau(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tableau.json"), []byte(minimal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tableau-window0.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := File{Dir: dir}
	tb, err := f.Tableau(context.Background(), Request{View: "tableau", Window: 1})
	if err != nil || tb.Project != "P" {
		t.Errorf("Tableau = %+v, %v", tb, err)
	}
	if _, err := f.Tableau(context.Background(), Request{View: "tableau", Window: 5}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file error = %v", err)
	}
	if _, err := f.Tableau(context.Background(), Request{View: "tableau", Window: 0}); err == nil || !strings.Contains(err.Error(), "tableau-window0.json") {
		t.Errorf("malformed file error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Tableau(ctx, Request{View: "tableau", Window: 1}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled error = %v", err)
	}
}
