package list

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodeFile decodes the fixture at path into the type its name gives.
func decodeFile(t *testing.T, path string) (head Head, err error) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	base := filepath.Base(path)
	switch {
	case strings.HasPrefix(base, "task-"):
		var v Task
		err = Decode(f, &v)
		return v.Head, err
	case strings.HasPrefix(base, "queue"):
		var v Queue
		err = Decode(f, &v)
		return v.Head, err
	case strings.HasPrefix(base, "blockage-"):
		var v Blockage
		err = Decode(f, &v)
		return v.Head, err
	case strings.HasPrefix(base, "history-"):
		var v History
		err = Decode(f, &v)
		return v.Head, err
	}
	t.Fatalf("no type for fixture %s", path)
	return head, nil
}

// T17: every fixture decodes, with a legend and a level.
func TestDecodeFixtures(t *testing.T) {
	var files []string
	for _, pattern := range []string{"testdata/*.json", "testdata/*/*.json"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) != 16 {
		t.Fatalf("found %d fixtures, want 16", len(files))
	}
	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			h, err := decodeFile(t, path)
			if err != nil {
				t.Fatal(err)
			}
			if len(h.Legend.Gates) == 0 || h.Project.ID == "" || h.Ref.Commit == "" {
				t.Errorf("head is incomplete: %+v", h)
			}
		})
	}
}

func TestDecodeContent(t *testing.T) {
	f, err := os.Open("testdata/task-e9c6.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var v Task
	if err := Decode(f, &v); err != nil {
		t.Fatal(err)
	}
	if v.Level != Detail || v.Task.ID != "e9c6" || v.Parent == nil || v.Parent.ID != "2034" || v.Parent.Order != 1 {
		t.Errorf("task = %+v", v)
	}
	if len(v.Junctions) != 10 || v.Junctions[4].Marks[1] != "reviewer" || !v.Junctions[4].Acts {
		t.Errorf("junctions = %+v", v.Junctions)
	}
}

func TestDecodeIgnoresUnknownKeys(t *testing.T) {
	var v Queue
	err := Decode(strings.NewReader(`{"level":"glance","surprise":{"x":1},"items":[{"kind":"ready","novel":true,"task":{"id":"a1b2","title":"T"}}]}`), &v)
	if err != nil {
		t.Fatal(err)
	}
	if v.Level != Glance || len(v.Items) != 1 || v.Items[0].Task.ID != "a1b2" {
		t.Errorf("queue = %+v", v)
	}
}

func TestDecodeRefusesUnknownLevel(t *testing.T) {
	var v History
	err := Decode(strings.NewReader(`{"level":"abyss"}`), &v)
	if err == nil || !strings.Contains(err.Error(), `unknown level "abyss"`) {
		t.Errorf("error = %v", err)
	}
}

func TestLevel(t *testing.T) {
	if Glance >= Detail || Detail >= Provenance {
		t.Error("the levels do not nest")
	}
	for l, want := range map[Level]string{Glance: "glance", Detail: "detail", Provenance: "provenance", Level(7): "level(7)"} {
		if got := l.String(); got != want {
			t.Errorf("Level(%d) = %q, want %q", int(l), got, want)
		}
	}
}
