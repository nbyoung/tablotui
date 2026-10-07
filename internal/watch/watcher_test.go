package watch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nbyoung/tablotui/internal/watch/watchtest"
)

// result is what one call of Next returned.
type result struct {
	st  State
	err error
}

// harness drives a watcher with the test clock. No test sleeps: each wait the
// watcher asks for arrives on clk.Asked, and clk.Tick releases it.
type harness struct {
	t      *testing.T
	dir    string
	w      *Watcher
	clk    *watchtest.Clock
	ctx    context.Context
	cancel context.CancelFunc
	pend   []chan result
}

// newHarness returns a harness for a watcher of src.
func newHarness(t *testing.T, src Source) *harness {
	t.Helper()
	clk := watchtest.NewClock()
	w := New(src)
	w.Clock = clk
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, dir: src.Dir, w: w, clk: clk, ctx: ctx, cancel: cancel}
	t.Cleanup(h.close)
	return h
}

// live returns a harness on the working tree of dir.
func live(t *testing.T, dir string) *harness {
	return newHarness(t, Source{Dir: dir})
}

// close cancels the context and waits for every Next that still runs.
func (h *harness) close() {
	h.cancel()
	for _, c := range h.pend {
		<-c
	}
}

// start runs Next in a goroutine and returns its result channel.
func (h *harness) start(prev State) chan result {
	c := make(chan result, 1)
	h.pend = append(h.pend, c)
	go func() {
		st, err := h.w.Next(h.ctx, prev)
		c <- result{st, err}
		close(c) // a later receive, as close makes, finds the channel done
	}()
	return c
}

// asked receives the next wait the watcher asks for and checks it.
func (h *harness) asked(want time.Duration) {
	h.t.Helper()
	if got := <-h.clk.Asked; got != want {
		h.t.Fatalf("the watcher waits %v, want %v", got, want)
	}
}

// tick releases the wait the watcher stands in.
func (h *harness) tick() { h.clk.Tick <- time.Time{} }

// none checks that no result has arrived.
func (h *harness) none(c chan result) {
	h.t.Helper()
	select {
	case r := <-c:
		h.t.Fatalf("Next returned %+v, want no change", r)
	default:
	}
}

// snap reads the state once.
func (h *harness) snap() State { return h.w.Snapshot(h.ctx) }

// change starts Next from the current state, runs act while the watcher stands
// in an interval, and returns the state Next delivers after one settle.
func (h *harness) change(act func()) State {
	h.t.Helper()
	prev := h.snap()
	c := h.start(prev)
	h.asked(h.w.Interval)
	act()
	h.tick()
	h.asked(h.w.Settle)
	h.tick()
	r := <-c
	if r.err != nil {
		h.t.Fatalf("Next: %v", r.err)
	}
	if r.st == prev {
		h.t.Fatal("Next returned the previous state")
	}
	return r.st
}

// quiet starts Next from the current state, runs act while the watcher stands
// in an interval, and checks that one more interval passes with nothing
// delivered.
func (h *harness) quiet(act func()) {
	h.t.Helper()
	c := h.start(h.snap())
	h.asked(h.w.Interval)
	act()
	h.tick()
	h.asked(h.w.Interval)
	h.none(c)
	h.cancel()
	if r := <-c; !errors.Is(r.err, context.Canceled) {
		h.t.Fatalf("Next returned %+v after the cancel", r)
	}
	h.pend = h.pend[:len(h.pend)-1]
	h.ctx, h.cancel = context.WithCancel(context.Background())
}

func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	watchtest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return watchtest.Git(t, dir, "rev-parse", "HEAD")
}

func TestT1CommitOnLooseRef(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	var want string
	st := h.change(func() { want = commit(t, dir, "second") })
	if st.Commit != want || st.Branch != "main" || st.Err != "" {
		t.Fatalf("got %+v, want commit %s on main", st, want)
	}
	if len(st.Commit) != 40 && len(st.Commit) != 64 {
		t.Fatalf("commit %q is no object id", st.Commit)
	}
}

func TestT2PackedRefs(t *testing.T) {
	dir := watchtest.Repo(t)
	pack := func() { watchtest.Git(t, dir, "pack-refs", "--all", "--prune") }
	pack()
	if _, err := os.Stat(filepath.Join(dir, ".git/refs/heads/main")); err == nil {
		t.Fatal("the ref is still loose")
	}
	h := live(t, dir)
	// A repack changes nothing a view reads.
	h.quiet(pack)
	// A commit over a packed ref is a change.
	var want string
	st := h.change(func() { want = commit(t, dir, "over packed") })
	if st.Commit != want {
		t.Fatalf("commit %s, want %s", st.Commit, want)
	}
	// A commit and a repack inside one interval are one change.
	st = h.change(func() { want = commit(t, dir, "and repack"); pack() })
	if st.Commit != want {
		t.Fatalf("commit %s, want %s", st.Commit, want)
	}
}

func TestT3Reftable(t *testing.T) {
	probe := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--ref-format=reftable", probe).CombinedOutput(); err != nil {
		t.Skipf("git lacks the reftable format: %s", strings.TrimSpace(string(out)))
	}
	dir := watchtest.Repo(t, "--ref-format=reftable")
	h := live(t, dir)
	var want string
	st := h.change(func() { want = commit(t, dir, "second") })
	if st.Commit != want || st.Branch != "main" {
		t.Fatalf("got %+v, want commit %s on main", st, want)
	}
}

func TestT4CheckoutAndDetach(t *testing.T) {
	dir := watchtest.Repo(t)
	watchtest.Git(t, dir, "branch", "same")
	h := live(t, dir)
	if st := h.change(func() { watchtest.Git(t, dir, "checkout", "-q", "same") }); st.Branch != "same" {
		t.Fatalf("branch %q, want same", st.Branch)
	}
	if st := h.change(func() { watchtest.Git(t, dir, "checkout", "-q", "--detach") }); st.Branch != "" {
		t.Fatalf("branch %q, want none when detached", st.Branch)
	}
	var want string
	st := h.change(func() { want = commit(t, dir, "detached") })
	if st.Commit != want || st.Branch != "" {
		t.Fatalf("got %+v, want commit %s detached", st, want)
	}
}

func TestT5LinkedWorktree(t *testing.T) {
	main := watchtest.Repo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	watchtest.Git(t, main, "worktree", "add", "-q", "-b", "side", wt)
	if fi, err := os.Stat(filepath.Join(wt, ".git")); err != nil || fi.IsDir() {
		t.Fatal(".git is not a file in the linked worktree")
	}
	h := live(t, wt)
	var want string
	st := h.change(func() { want = commit(t, wt, "in worktree") })
	if st.Commit != want || st.Branch != "side" {
		t.Fatalf("got %+v, want commit %s on side", st, want)
	}
	// The worktree's own .tableaux edit is a change.
	h.change(func() { watchtest.Write(t, filepath.Join(wt, ".tableaux/tasks/bbbb.yaml"), "title: B\n") })
	// A commit in the main checkout moves the refs alone.
	before := h.snap()
	after := h.change(func() { commit(t, main, "in main") })
	if after.Refs == before.Refs {
		t.Fatal("the refs digest did not move")
	}
	after.Refs = before.Refs
	if after != before {
		t.Fatalf("more than the refs moved: %+v, then %+v", before, after)
	}
	// An edit of the main checkout's .tableaux is none.
	h.quiet(func() { watchtest.Write(t, filepath.Join(main, ".tableaux/tasks/cccc.yaml"), "title: C\n") })
}

func TestT6Submodule(t *testing.T) {
	sub := watchtest.Repo(t)
	super := watchtest.Repo(t)
	watchtest.Git(t, super, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "libs/sub")
	watchtest.Git(t, super, "commit", "-q", "-m", "add submodule")
	inner := filepath.Join(super, "libs/sub")
	if fi, err := os.Stat(filepath.Join(inner, ".git")); err != nil || fi.IsDir() {
		t.Fatal(".git is not a file in the submodule")
	}

	// A watcher inside the submodule sees its commit.
	var want string
	hs := live(t, inner)
	if st := hs.change(func() { want = commit(t, inner, "in submodule") }); st.Commit != want {
		t.Fatalf("commit %s, want %s", st.Commit, want)
	}

	// In the parent, the submodule's checkout moved already; the watcher that
	// starts now reads it as its baseline, and a further move is a change.
	h := live(t, super)
	h.w.SetSubmodules([]string{"libs/sub"})
	before := h.snap()
	if before.Modules == "" {
		t.Fatal("the state holds no submodule digest")
	}
	st := h.change(func() { want = commit(t, inner, "moves the checkout") })
	if st.Commit != before.Commit || st.Modules == before.Modules {
		t.Fatalf("got %+v, want only the submodule digest moved from %+v", st, before)
	}
	// A pin staged and not committed is no change by itself.
	h.quiet(func() { watchtest.Git(t, super, "add", "libs/sub") })
	// The commit that records the pin is one.
	var parent string
	st = h.change(func() { parent = commit(t, super, "record the pin") })
	if st.Commit != parent {
		t.Fatalf("commit %s, want %s", st.Commit, parent)
	}
	// Without the list, a moved checkout is no change.
	h.w.SetSubmodules(nil)
	h.quiet(func() { commit(t, inner, "unseen") })
}

func TestT7WorkingTreeEdits(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	status := filepath.Join(dir, ".tableaux/status/aaaa.yaml")
	extra := filepath.Join(dir, ".tableaux/tasks/bbbb.yaml")

	h.change(func() { watchtest.Write(t, status, "gate: defined\nstate: at_risk\n") })
	h.change(func() { watchtest.Write(t, extra, "title: B\n") })
	h.change(func() { _ = os.Remove(extra) })

	// An edit that keeps the size and moves the time.
	mt := time.Now().Add(time.Hour)
	h.change(func() {
		watchtest.Write(t, status, "gate: defined\nstate: nominal\n")
		if err := os.Chtimes(status, mt, mt); err != nil {
			t.Fatal(err)
		}
	})
	same := mt.Add(time.Hour)
	h.change(func() {
		watchtest.Write(t, status, "gate: defined\nstate: nominaL\n")
		if err := os.Chtimes(status, same, same); err != nil {
			t.Fatal(err)
		}
	})

	// A file outside .tableaux, a directory .tableauxish and git add -A.
	h.quiet(func() {
		watchtest.Write(t, filepath.Join(dir, "README"), "outside\n")
		watchtest.Write(t, filepath.Join(dir, ".tableauxish/x.yaml"), "x\n")
		watchtest.Git(t, dir, "add", "-A")
	})

	// A removed .tableaux reads absent.
	st := h.change(func() {
		if err := os.RemoveAll(filepath.Join(dir, ".tableaux")); err != nil {
			t.Fatal(err)
		}
	})
	if st.Tree != "absent" || st.Err != "" {
		t.Fatalf("got %+v, want tree absent", st)
	}
}

func TestT8Burst(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	add := func(n string) { watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/"+n+".yaml"), "title: "+n+"\n") }
	prev := h.snap()
	c := h.start(prev)
	h.asked(h.w.Interval)
	add("b")
	h.tick()
	h.asked(h.w.Settle)
	add("c")
	h.tick()
	h.asked(h.w.Settle)
	add("d")
	h.tick()
	h.asked(h.w.Settle)
	h.none(c)
	h.tick()
	r := <-c
	if r.err != nil || r.st != h.snap() || r.st == prev {
		t.Fatalf("Next returned %+v, %v", r.st, r.err)
	}
	// The burst is spent: one quiet interval follows.
	h.quiet(func() {})
}

func TestT9RevertWhileSettling(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	status := filepath.Join(dir, ".tableaux/status/aaaa.yaml")
	fi, err := os.Stat(status)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(status)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(orig), "defined", "definex", 1)
	c := h.start(h.snap())
	h.asked(h.w.Interval)
	watchtest.Write(t, status, edited)
	if err := os.Chtimes(status, fi.ModTime().Add(time.Hour), fi.ModTime().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	h.tick()
	h.asked(h.w.Settle)
	watchtest.Write(t, status, string(orig))
	if err := os.Chtimes(status, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	h.tick()
	h.asked(h.w.Settle) // the read differed from the one before: settle again
	h.tick()
	h.asked(h.w.Interval) // the state is the old one: nothing to report
	h.none(c)
}

func TestT10StreamReportsAfterMaxSettle(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	edit := func(n int) {
		watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/n"+strings.Repeat("x", n)+".yaml"), "title: n\n")
	}
	c := h.start(h.snap())
	h.asked(h.w.Interval)
	edit(0)
	h.tick()
	for i := 1; i <= 8; i++ {
		h.asked(h.w.Settle)
		h.none(c)
		edit(i)
		h.tick()
	}
	r := <-c
	if r.err != nil || r.st != h.snap() {
		t.Fatalf("Next returned %+v, %v after the eighth settle", r.st, r.err)
	}
	if n := len(h.clk.Asked); n != 0 {
		t.Fatalf("%d waits follow the delivery", n)
	}
}

func TestT11Poke(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	prev := h.snap()

	// A poke while the watcher stands in an interval.
	c := h.start(prev)
	h.asked(h.w.Interval)
	h.w.Poke()
	r := <-c
	if r.err != nil || r.st != prev {
		t.Fatalf("poked Next returned %+v, %v", r.st, r.err)
	}

	// A poke before Next, and a second that does not queue.
	h.w.Poke()
	h.w.Poke()
	if r := <-h.start(prev); r.err != nil || r.st != prev {
		t.Fatalf("Next after a poke returned %+v, %v", r.st, r.err)
	}
	if n := len(h.clk.Asked); n != 0 {
		t.Fatalf("a poke left %d waits", n)
	}
	c = h.start(prev)
	h.asked(h.w.Interval)
	h.none(c)
}

// A poke during a settle waits for it: the settle completes and delivers the
// change, and the Next that follows returns at once, with no wait of either
// kind (the owner's ruling of 2026-10-06 on the loop's steps 1, 3 and 5).
func TestPokeDuringASettleWaitsForIt(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	prev := h.snap()
	c := h.start(prev)
	h.asked(h.w.Interval)
	watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/b.yaml"), "title: b\n")
	h.tick()
	h.asked(h.w.Settle)
	h.w.Poke()
	h.none(c)
	h.tick()
	r := <-c
	if r.err != nil || r.st == prev {
		t.Fatalf("the settle delivered %+v, %v", r.st, r.err)
	}
	if r := <-h.start(r.st); r.err != nil {
		t.Fatalf("Next after the settle returned %v", r.err)
	}
	if n := len(h.clk.Asked); n != 0 {
		t.Fatalf("the poke left %d waits", n)
	}
}

func TestT12BrokenRepositoryHeals(t *testing.T) {
	dir := watchtest.Repo(t)
	h := live(t, dir)
	git, off := filepath.Join(dir, ".git"), filepath.Join(dir, ".git.off")
	st := h.change(func() {
		if err := os.Rename(git, off); err != nil {
			t.Fatal(err)
		}
	})
	if st.Err == "" || st.Commit != "" || st.Refs != "" || st.Tree != "" {
		t.Fatalf("got %+v, want an Err alone", st)
	}
	if strings.HasPrefix(st.Err, "fatal: ") {
		t.Fatalf("Err %q keeps git's prefix", st.Err)
	}
	st = h.change(func() {
		if err := os.Rename(off, git); err != nil {
			t.Fatal(err)
		}
	})
	if st.Err != "" || st.Commit == "" {
		t.Fatalf("got %+v, want the repository healed", st)
	}
}

func TestT13Cancel(t *testing.T) {
	dir := watchtest.Repo(t)

	h := live(t, dir)
	prev := h.snap()
	c := h.start(prev)
	h.asked(h.w.Interval)
	h.cancel()
	if r := <-c; r.st != prev || !errors.Is(r.err, context.Canceled) {
		t.Fatalf("from the interval: %+v, %v", r.st, r.err)
	}

	h = live(t, dir)
	c = h.start(prev)
	h.asked(h.w.Interval)
	watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/bbbb.yaml"), "title: B\n")
	h.tick()
	h.asked(h.w.Settle)
	h.cancel()
	if r := <-c; r.st != prev || !errors.Is(r.err, context.Canceled) {
		t.Fatalf("from the settle: %+v, %v", r.st, r.err)
	}
}

func TestT14Ref(t *testing.T) {
	dir := watchtest.Repo(t)
	watchtest.Git(t, dir, "branch", "topic")
	next := commit(t, dir, "ahead") // main moves; topic stays behind
	src, err := Open(context.Background(), dir, "topic")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, src)
	st := h.snap()
	if st.Tree != "" || st.Branch != "" || st.Modules != "" || st.Commit == "" || st.Err != "" {
		t.Fatalf("got %+v, want a commit and no tree, branch or modules", st)
	}
	if want := watchtest.Git(t, dir, "rev-parse", "topic"); st.Commit != want || st.Commit == next {
		t.Fatalf("commit %s, want topic's", st.Commit)
	}
	// A working-tree edit is none.
	h.quiet(func() { watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/bbbb.yaml"), "title: B\n") })
	// A moved ref is a change.
	moved := h.change(func() { watchtest.Git(t, dir, "update-ref", "refs/heads/topic", next) })
	if moved.Commit != next || moved.Tree != "" || moved.Branch != "" {
		t.Fatalf("got %+v, want topic at %s", moved, next)
	}
	// A deleted ref gives Err.
	gone := h.change(func() { watchtest.Git(t, dir, "branch", "-q", "-D", "topic") })
	if gone.Err == "" || gone.Commit != "" || gone.Refs != "" {
		t.Fatalf("got %+v, want an Err alone", gone)
	}
}

func TestT15Open(t *testing.T) {
	dir := watchtest.Repo(t)
	outside := t.TempDir()
	tests := []struct {
		name  string
		dir   string
		at    string
		usage bool
		err   string // a substring of the message; empty for success
	}{
		{"none", dir, "", false, ""},
		{"branch", dir, "main", false, ""},
		{"head", dir, "HEAD", false, ""},
		{"hyphen", dir, "-x", true, "usage: --ref -x: name one commit"},
		{"option", dir, "--all", true, "usage: --ref --all: name one commit"},
		{"range", dir, "main..HEAD", true, "usage: --ref main..HEAD: name one commit"},
		{"unknown", dir, "nope", false, "--ref nope: no such commit"},
		{"blob", dir, "HEAD:README", false, "--ref HEAD:README: no such commit"},
		{"outside", outside, "", false, outside + ": "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, err := Open(context.Background(), tc.dir, tc.at)
			if tc.err == "" {
				if err != nil {
					t.Fatal(err)
				}
				if src.Dir != tc.dir || src.Ref != tc.at || src.Live() != (tc.at == "") {
					t.Fatalf("got %+v", src)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("got %v, want %q", err, tc.err)
			}
			if errors.Is(err, ErrUsage) != tc.usage {
				t.Fatalf("errors.Is(%v, ErrUsage) is %v", err, !tc.usage)
			}
		})
	}
	t.Run("relative", func(t *testing.T) {
		t.Chdir(dir)
		src, err := Open(context.Background(), ".", "")
		if err != nil || !filepath.IsAbs(src.Dir) {
			t.Fatalf("got %+v, %v", src, err)
		}
	})
}

// recorder wraps a runner and keeps each call.
type recorder struct {
	mu    sync.Mutex
	calls [][]string
	next  runner
}

func (r *recorder) run(ctx context.Context, dir string, args ...string) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, append([]string{dir}, args...))
	r.mu.Unlock()
	return r.next(ctx, dir, args...)
}

func TestT16SnapshotProcesses(t *testing.T) {
	dir := watchtest.Repo(t)
	w := New(Source{Dir: dir})
	rec := &recorder{next: w.run}
	w.run = rec.run
	if st := w.Snapshot(context.Background()); st.Err != "" {
		t.Fatal(st.Err)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("a snapshot ran %d git processes, want 2: %v", len(rec.calls), rec.calls)
	}

	// One more process for each submodule the caller names.
	rec.calls = nil
	w.SetSubmodules([]string{"a", "b"})
	w.Snapshot(context.Background())
	if len(rec.calls) != 4 {
		t.Fatalf("a snapshot with two submodules ran %d processes, want 4", len(rec.calls))
	}

	// With --ref the list is not read.
	wr := New(Source{Dir: dir, Ref: "main"})
	wr.SetSubmodules([]string{"a"})
	rec = &recorder{next: wr.run}
	wr.run = rec.run
	wr.Snapshot(context.Background())
	if len(rec.calls) != 2 {
		t.Fatalf("a snapshot at a ref ran %d processes, want 2", len(rec.calls))
	}
}

func TestT16Environment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub git is a shell script")
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git on the path")
	}
	dir := watchtest.Repo(t)
	bin := t.TempDir()
	logPath := filepath.Join(bin, "env.log")
	script := "#!/bin/sh\n" +
		"{ echo \"GIT_OPTIONAL_LOCKS=$GIT_OPTIONAL_LOCKS\"; echo \"GIT_TERMINAL_PROMPT=$GIT_TERMINAL_PROMPT\"; echo \"LC_ALL=$LC_ALL\"; echo \"ARGS=$*\"; } >> \"" + logPath + "\"\n" +
		"exec \"" + gitBin + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if st := New(Source{Dir: dir}).Snapshot(context.Background()); st.Err != "" {
		t.Fatal(st.Err)
	}
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("the stub git did not run: %v", err)
	}
	got := string(b)
	for _, want := range []string{"GIT_OPTIONAL_LOCKS=0\n", "GIT_TERMINAL_PROMPT=0\n", "LC_ALL=C\n", "ARGS=-C " + dir + " rev-parse HEAD --abbrev-ref HEAD\n", "ARGS=-C " + dir + " for-each-ref "} {
		if strings.Count(got, want) == 0 {
			t.Errorf("the log lacks %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "GIT_OPTIONAL_LOCKS=0\n"); n != 2 {
		t.Errorf("%d processes ran with the settings, want 2", n)
	}
}
