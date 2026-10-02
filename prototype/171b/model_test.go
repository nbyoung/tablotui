package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type stubLoader struct {
	v   View
	err error
}

func (s *stubLoader) Load(context.Context, Source) (View, error) { return s.v, s.err }

func newTestModel(src Source, l Loader) Model {
	m := NewModel(context.Background(), src, l, &Watcher{Dir: ".", Ref: "HEAD", Interval: time.Hour, Settle: time.Hour}, Fingerprint{Commit: "c0"})
	t0 := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	m.now = func() time.Time { t0 = t0.Add(time.Minute); return t0 }
	return m
}

func upd(m Model, msg tea.Msg) (Model, tea.Cmd) {
	n, c := m.Update(msg)
	return n.(Model), c
}

func TestLoadThenChangeThenReload(t *testing.T) {
	l := &stubLoader{v: View{Commit: "aaa", Ref: "HEAD", Digest: "d1", Files: 3}}
	m := newTestModel(LiveSource(), l)
	if !strings.Contains(m.View(), "loading") {
		t.Fatal("no loading state")
	}
	m, _ = upd(m, m.loadCmd(1)())
	if m.reloads != 0 || !strings.Contains(m.View(), "aaa") {
		t.Fatalf("first load: %d\n%s", m.reloads, m.View())
	}
	l.v = View{Commit: "bbb", Ref: "HEAD", Digest: "d2", Files: 4}
	m, cmd := upd(m, ChangedMsg{FP: Fingerprint{Commit: "bbb"}})
	if cmd == nil || m.fp.Commit != "bbb" || m.seq != 2 {
		t.Fatalf("change: cmd %v fp %+v seq %d", cmd != nil, m.fp, m.seq)
	}
	m, _ = upd(m, m.loadCmd(2)())
	out := m.View()
	for _, want := range []string{"bbb", "d2 (4 files)", "reloads  1", "09:02:00", "live"} {
		if !strings.Contains(out, want) {
			t.Errorf("view lacks %q\n%s", want, out)
		}
	}
}

func TestFailedReloadKeepsLastGoodView(t *testing.T) {
	l := &stubLoader{v: View{Commit: "aaa", Digest: "d1", Files: 3}}
	m := newTestModel(LiveSource(), l)
	m, _ = upd(m, m.loadCmd(1)())
	l.err = errors.New("fatal: bad object")
	m, _ = upd(m, ChangedMsg{FP: Fingerprint{Err: "x"}})
	m, _ = upd(m, m.loadCmd(2)())
	out := m.View()
	if !strings.Contains(out, "aaa") || !strings.Contains(out, "fatal: bad object") || m.reloads != 0 {
		t.Fatalf("error view:\n%s", out)
	}
	l.err = nil
	l.v = View{Commit: "ccc", Digest: "d3", Files: 3}
	m, _ = upd(m, ChangedMsg{FP: Fingerprint{Commit: "ccc"}})
	m, _ = upd(m, m.loadCmd(3)())
	if strings.Contains(m.View(), "error") || !strings.Contains(m.View(), "ccc") || m.reloads != 1 {
		t.Fatalf("recovery:\n%s", m.View())
	}
}

func TestStaleLoadIsIgnored(t *testing.T) {
	m := newTestModel(LiveSource(), &stubLoader{})
	m, _ = upd(m, ChangedMsg{FP: Fingerprint{Commit: "b"}}) // seq 2
	m, _ = upd(m, LoadedMsg{Seq: 1, View: View{Commit: "old"}})
	if m.loaded {
		t.Fatal("a stale load replaced the view")
	}
	m, _ = upd(m, LoadedMsg{Seq: 2, View: View{Commit: "new"}})
	m, _ = upd(m, LoadedMsg{Seq: 1, View: View{Commit: "old"}})
	if m.view.Commit != "new" {
		t.Fatalf("view %q", m.view.Commit)
	}
}

func TestPinnedDoesNotWatch(t *testing.T) {
	src := Source{Ref: "v1", Commit: "abc"}
	m := newTestModel(src, &stubLoader{v: View{Commit: "abc"}})
	if m.watchCmd(m.fp) != nil {
		t.Fatal("pinned model arms a watcher")
	}
	_, cmd := upd(m, ChangedMsg{FP: Fingerprint{Commit: "z"}})
	if cmd == nil {
		t.Fatal("a stray change should still reload")
	}
	if !strings.Contains(m.View(), "pinned to v1") {
		t.Fatalf("view:\n%s", m.View())
	}
}

func TestWatchCmdDeliversAndRearms(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, false)
	ctx, cancel := context.WithTimeout(context.Background(), generous)
	defer cancel()
	base := w.Snapshot(ctx)
	m := NewModel(ctx, LiveSource(), GitLoader{Dir: dir}, w, base)
	done := make(chan tea.Msg, 1)
	go func() { done <- m.watchCmd(base)() }()
	time.Sleep(50 * time.Millisecond)
	commit(t, dir, "x")
	msg, ok := (<-done).(ChangedMsg)
	if !ok || msg.FP == base {
		t.Fatalf("msg %+v", msg)
	}
	m, cmd := upd(m, msg)
	if cmd == nil || m.fp != msg.FP {
		t.Fatal("not re-armed")
	}
	// A cancelled context ends the waiter without a message.
	cancel()
	if got := m.watchCmd(m.fp)(); got != nil {
		t.Fatalf("cancelled waiter returned %v", got)
	}
}

func TestQuitAndResize(t *testing.T) {
	m := newTestModel(LiveSource(), &stubLoader{})
	m, _ = upd(m, tea.WindowSizeMsg{Width: 30, Height: 10})
	m.err = errors.New(strings.Repeat("e", 80))
	for _, line := range strings.Split(m.View(), "\n") {
		if len(line) > 30 {
			t.Fatalf("line wider than the window: %q", line)
		}
	}
	if _, cmd := upd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Fatal("q does not quit")
	}
}

func TestSample(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, true)
	ctx := context.Background()
	src := LiveSource()
	m := NewModel(ctx, src, GitLoader{Dir: dir}, w, w.Snapshot(ctx))
	m, _ = upd(m, m.loadCmd(1)())
	write(t, dir+"/.tableaux/status/aaaa.yaml", "gate: defined\nstate: at_risk\n")
	commit(t, dir, "second")
	m, _ = upd(m, ChangedMsg{FP: w.Snapshot(ctx)})
	m, _ = upd(m, m.loadCmd(2)())
	t.Logf("live after a commit and an edit:\n%s", m.View())
}
