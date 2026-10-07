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

func TestListName(t *testing.T) {
	f := File{}
	cases := []struct {
		r    ListRequest
		want string
	}{
		{ListRequest{View: "task", Task: "e9c6"}, "task-e9c6.json"},
		{ListRequest{View: "task", Task: "e9c6", Level: "detail"}, "task-e9c6.json"},
		{ListRequest{View: "task", Task: "e9c6", Level: "provenance"}, "task-e9c6-provenance.json"},
		{ListRequest{View: "queue"}, "queue.json"},
		{ListRequest{View: "queue", Level: "provenance"}, "queue-provenance.json"},
		{ListRequest{View: "blockage", Task: "437e", Level: "provenance"}, "blockage-437e-provenance.json"},
		{ListRequest{View: "history", Task: "5fe3"}, "history-5fe3.json"},
	}
	for _, c := range cases {
		if got := f.ListName(c.r); got != c.want {
			t.Errorf("ListName(%+v) = %s, want %s", c.r, got, c.want)
		}
	}
}

// T16: source.File as Lists.
func TestFileLists(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("task-aaaa.json", `{"level":"detail","task":{"id":"aaaa","title":"A"}}`)
	write("task-aaaa-provenance.json", `{"level":"provenance","task":{"id":"aaaa","title":"A"}}`)
	write("queue.json", `{"level":"glance","params":{"person":"ada@example.org"}}`)
	write("blockage-aaaa.json", `{"level":"detail","causes":[{"kind":"status","task":{"id":"aaaa"}}]}`)
	write("history-aaaa.json", `{"level":"detail","events":2}`)
	write("history-bbbb.json", `{`)
	write("queue-provenance.json", `{"level":"nonsense"}`)

	var l Lists = File{Dir: dir}
	ctx := context.Background()

	if v, err := l.Task(ctx, ListRequest{Task: "aaaa"}); err != nil || v.Task.ID != "aaaa" || v.Level.String() != "detail" {
		t.Errorf("Task = %+v, %v", v, err)
	}
	if v, err := l.Task(ctx, ListRequest{Task: "aaaa", Level: "provenance"}); err != nil || v.Level.String() != "provenance" {
		t.Errorf("Task at provenance = %+v, %v", v, err)
	}
	if v, err := l.Queue(ctx, ListRequest{}); err != nil || v.Params.Person != "ada@example.org" {
		t.Errorf("Queue = %+v, %v", v, err)
	}
	if v, err := l.Blockage(ctx, ListRequest{Task: "aaaa"}); err != nil || len(v.Causes) != 1 {
		t.Errorf("Blockage = %+v, %v", v, err)
	}
	if v, err := l.History(ctx, ListRequest{Task: "aaaa"}); err != nil || v.Events != 2 {
		t.Errorf("History = %+v, %v", v, err)
	}

	// The method, not the request, names the view.
	if v, err := l.Task(ctx, ListRequest{View: "queue", Task: "aaaa"}); err != nil || v.Task.ID != "aaaa" {
		t.Errorf("Task with a stray View = %+v, %v", v, err)
	}

	// A missing file.
	if _, err := l.Task(ctx, ListRequest{Task: "zzzz"}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file error = %v", err)
	}
	// A file that does not parse names itself.
	if _, err := l.History(ctx, ListRequest{Task: "bbbb"}); err == nil || !strings.Contains(err.Error(), "history-bbbb.json") {
		t.Errorf("malformed file error = %v", err)
	}
	if _, err := l.Queue(ctx, ListRequest{Level: "provenance"}); err == nil || !strings.Contains(err.Error(), "queue-provenance.json") || !strings.Contains(err.Error(), "unknown level") {
		t.Errorf("unknown level error = %v", err)
	}
	// A cancelled context.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	for name, call := range map[string]func() error{
		"Task":     func() error { _, err := l.Task(cctx, ListRequest{Task: "aaaa"}); return err },
		"Queue":    func() error { _, err := l.Queue(cctx, ListRequest{}); return err },
		"Blockage": func() error { _, err := l.Blockage(cctx, ListRequest{Task: "aaaa"}); return err },
		"History":  func() error { _, err := l.History(cctx, ListRequest{Task: "aaaa"}); return err },
	} {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a cancelled context = %v", name, err)
		}
	}
}
