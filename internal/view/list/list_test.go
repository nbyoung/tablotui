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
			if len(h.Legend.Gates) == 0 || h.Project.Root == "" || h.Source.Commit == "" {
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
	if v.Level != Detail || v.Task.ID != "e9c6" || v.Parent == nil || v.Parent.ID != "2034" || v.Parent.Order == nil || *v.Parent.Order != 1 {
		t.Errorf("task = %+v", v)
	}
	if len(v.Junctions) != 10 || v.Junctions[4].Marks[1] != "reviewer" || !v.Junctions[4].Acts {
		t.Errorf("junctions = %+v", v.Junctions)
	}
}

func TestDecodeIgnoresUnknownKeys(t *testing.T) {
	var v Queue
	err := Decode(strings.NewReader(`{"command":"queue","surprise":1,"params":{"level":"glance","novel":2},"data":{"surprise":{"x":1},"items":[{"kind":"ready","novel":true,"task":{"id":"a1b2","title":"T"}}]}}`), &v)
	if err != nil {
		t.Fatal(err)
	}
	if v.Level != Glance || len(v.Items) != 1 || v.Items[0].Task.ID != "a1b2" {
		t.Errorf("queue = %+v", v)
	}
}

func TestDecodeRefusesUnknownLevel(t *testing.T) {
	var v History
	err := Decode(strings.NewReader(`{"command":"history","params":{"level":"abyss"},"data":{}}`), &v)
	if err == nil || !strings.Contains(err.Error(), `unknown level "abyss"`) {
		t.Errorf("error = %v", err)
	}
}

func TestDecodeRefusesAnotherEnvelope(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"command":"queue","data":{}}`, `the command "queue", not "task"`},
		{`{"command":"task","data":null}`, "no data"},
		{`{"command":"task"}`, "no data"},
		{`{`, "unexpected EOF"},
	}
	for _, c := range cases {
		var v Task
		if err := Decode(strings.NewReader(c.in), &v); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Decode(%s) = %v, want %q", c.in, err, c.want)
		}
	}
}

func TestDecodeHead(t *testing.T) {
	var v Blockage
	in := `{"command":"blockage","source":{"ref":"main","commit":"abc","date":"2026-10-05"},
"project":{"root":"437e","title":"P"},"params":{"task":"e9c6","person":null,"level":"provenance"},
"data":{"legend":{"gates":[{"key":"defined","symbol":"📝"}]},"causes":[]}}`
	if err := Decode(strings.NewReader(in), &v); err != nil {
		t.Fatal(err)
	}
	want := Head{Level: Provenance, Project: Project{Root: "437e", Title: "P"}, Source: Source{Ref: "main", Commit: "abc", Date: "2026-10-05"},
		Params: Params{Task: "e9c6"}}
	if v.Level != want.Level || v.Project != want.Project || v.Source != want.Source || v.Params != want.Params || len(v.Legend.Gates) != 1 {
		t.Errorf("head = %+v", v.Head)
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
