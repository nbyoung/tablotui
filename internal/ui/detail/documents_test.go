package detail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T1: the lines of every view at every level, and the wrap.
func TestGoldenDocuments(t *testing.T) {
	cases := []struct {
		golden string
		pane   string
		dir    string // under testdata
		task   string // the selection; "" none
		keys   string // the viewer's keys after the first load
		width  int
	}{
		{"task-e9c6.glance.txt", "task", "", "e9c6", "d", 78},
		{"task-e9c6.detail.txt", "task", "", "e9c6", "", 78},
		{"task-e9c6.detail.38.txt", "task", "", "e9c6", "", 38},
		{"task-e9c6.provenance.txt", "task", "", "e9c6", "p", 78},
		{"task-5fe3.provenance.txt", "task", "", "5fe3", "p", 78},
		{"task-595e.provenance.txt", "task", "", "595e", "p", 78},
		{"queue.glance.txt", "queue", "", "e9c6", "d", 78},
		{"queue.detail.txt", "queue", "", "bb7c", "", 78},
		{"queue.unfolded.txt", "queue", "", "", "d f", 78},
		{"queue-kinds.detail.txt", "queue", "kinds", "", "", 78},
		{"queue-empty.txt", "queue", "empty", "", "", 78},
		{"blockage-437e.glance.txt", "blockage", "", "437e", "d", 78},
		{"blockage-437e.detail.txt", "blockage", "", "437e", "", 78},
		{"blockage-437e.provenance.txt", "blockage", "", "437e", "p", 78},
		{"blockage-e9c6.detail.txt", "blockage", "", "e9c6", "", 78},
		{"history-e9c6.glance.txt", "history", "", "e9c6", "d", 78},
		{"history-e9c6.detail.txt", "history", "", "e9c6", "", 78},
		{"history-e9c6.provenance.txt", "history", "", "e9c6", "p", 78},
		{"history-e9c6.provenance.38.txt", "history", "", "e9c6", "p", 38},
		{"history-595e.detail.txt", "history", "", "595e", "", 78},
		{"history-5fe3.txt", "history", "", "5fe3", "", 78},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			x := pane(t, c.pane, filepath.Join("testdata", c.dir), c.width+2, 30)
			x.open().sel(c.task, "").keys(c.keys)
			if got, want := x.doc(c.width), golden(t, c.golden); got != want {
				t.Errorf("document differs from %s:\n%s\nwant:\n%s", c.golden, got, want)
			}
		})
	}
	// Every golden document of the drafts has a case.
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, c := range cases {
		known[c.golden] = true
	}
	for _, f := range files {
		name := filepath.Base(f)
		if !known[name] && !screenName(name) && name != "layout.txt" {
			t.Errorf("no test reads %s", name)
		}
	}
	if _, err := os.Stat("testdata/layout.txt"); err != nil {
		t.Error(err)
	}
}

// screenName reports a golden screen: its name ends in <w>x<h>.txt.
func screenName(name string) bool {
	base := strings.TrimSuffix(name, ".txt")
	i := strings.LastIndex(base, "-")
	if i < 0 {
		return false
	}
	w, h, ok := strings.Cut(base[i+1:], "x")
	return ok && w != "" && h != "" && strings.Trim(w+h, "0123456789") == ""
}
