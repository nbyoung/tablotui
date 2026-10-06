package refresh

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/watch"
	"github.com/nbyoung/tablotui/internal/watch/watchtest"
)

// script is a Watcher that a test drives: Next returns the states the test
// sends and records the state it armed against.
type script struct {
	mu    sync.Mutex
	snap  watch.State
	prevs []watch.State
	feed  chan watch.State
	pokes atomic.Int32
}

func newScript(snap watch.State) *script {
	return &script{snap: snap, feed: make(chan watch.State, 8)}
}

func (s *script) Snapshot(context.Context) watch.State { return s.snap }

func (s *script) Next(ctx context.Context, prev watch.State) (watch.State, error) {
	s.mu.Lock()
	s.prevs = append(s.prevs, prev)
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return prev, ctx.Err()
	case st := <-s.feed:
		return st, nil
	}
}

func (s *script) Poke() { s.pokes.Add(1) }

func (s *script) armed() []watch.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]watch.State(nil), s.prevs...)
}

// stepper is a clock that moves one second at each call.
type stepper struct{ t time.Time }

func (c *stepper) now() time.Time {
	c.t = c.t.Add(time.Second)
	return c.t
}

func newStepper() *stepper { return &stepper{t: time.Date(2026, 10, 6, 14, 0, 0, 0, time.Local)} }

var (
	live  = watch.Source{Dir: "/p"}
	state = func(c string) watch.State { return watch.State{Commit: c, Branch: "main"} }
)

// run runs a command by hand, as Bubble Tea does, and returns its message.
func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	return cmd()
}

func TestT17Alternation(t *testing.T) {
	s1, s2 := state("aaaaaaa1"), state("bbbbbbb2")
	w := newScript(s1)
	loads := 0
	m := New(context.Background(), live, w, newStepper().now, func(_ context.Context, st watch.State) (string, error) {
		loads++
		return "data of " + st.Commit, nil
	})

	// Init reads the first state.
	msg := run(t, m.Init())
	if c, ok := msg.(changed); !ok || c.state != s1 {
		t.Fatalf("Init yields %#v, want changed %v", msg, s1)
	}
	// changed sets Loading and returns the load.
	m, cmd := m.Update(msg)
	if !m.Status().Loading || m.Status().Good {
		t.Fatalf("status %+v after changed", m.Status())
	}
	loaded, ok := run(t, cmd).(Loaded[string])
	if !ok || loaded.State != s1 || loaded.Data != "data of aaaaaaa1" || loaded.Err != nil {
		t.Fatalf("the load yields %#v", loaded)
	}
	// Loaded clears Loading, sets Good and returns the wait.
	m, cmd = m.Update(loaded)
	st := m.Status()
	if st.Loading || !st.Good || st.State != s1 || st.Err != nil || st.Loaded.IsZero() {
		t.Fatalf("status %+v after the load", st)
	}
	w.feed <- s2
	if c, ok := run(t, cmd).(changed); !ok || c.state != s2 {
		t.Fatal("the wait does not report the next state")
	}
	// The wait armed against the state the load read.
	if got := w.armed(); !reflect.DeepEqual(got, []watch.State{s1}) {
		t.Fatalf("armed against %v, want [%v]", got, s1)
	}
	m, cmd = m.Update(changed{s2})
	loaded = run(t, cmd).(Loaded[string])
	m, cmd = m.Update(loaded)
	w.feed <- state("cccccccc3")
	run(t, cmd)
	if got := w.armed(); !reflect.DeepEqual(got, []watch.State{s1, s2}) {
		t.Fatalf("armed against %v, want [%v %v]", got, s1, s2)
	}
	if loads != 2 || m.Status().State != s2 {
		t.Fatalf("%d loads, state %v", loads, m.Status().State)
	}
	// Force pokes the watcher.
	m.Force()
	if w.pokes.Load() != 1 {
		t.Fatal("Force did not poke")
	}
}

func TestT18FailedReloadKeepsTheView(t *testing.T) {
	s1, s2, s3, s4 := state("aaaaaaa1"), state("bbbbbbb2"), state("cccccccc3"), state("dddddddd4")
	w := newScript(s1)
	clk := newStepper()
	fail := false
	m := New(context.Background(), live, w, clk.now, func(_ context.Context, st watch.State) (string, error) {
		if fail {
			return "", errors.New("status/9f31.yaml:2:8: S3 unknown state \"nomnal\"\nmore")
		}
		return st.Commit, nil
	})
	step := func(st watch.State) Loaded[string] {
		t.Helper()
		var cmd tea.Cmd
		m, cmd = m.Update(changed{st})
		l := run(t, cmd).(Loaded[string])
		m, _ = m.Update(l)
		return l
	}
	step(s1)
	good := m.Status()
	if !good.Good || good.Err != nil || !good.Failed.IsZero() {
		t.Fatalf("status %+v after a good load", good)
	}

	fail = true
	l := step(s2)
	if l.Err == nil || l.Data != "" {
		t.Fatalf("the failed load yields %#v", l)
	}
	st := m.Status()
	if !st.Good || st.State != s1 || st.Loaded != good.Loaded || st.Err == nil || st.Failed.IsZero() || st.Loading {
		t.Fatalf("status %+v after a failure", st)
	}
	failed := st.Failed

	// A second failure keeps the time the run began.
	step(s3)
	if st := m.Status(); st.Failed != failed || st.State != s1 || st.Loaded != good.Loaded {
		t.Fatalf("status %+v after a second failure", st)
	}
	if got := w.armed(); len(got) != 0 {
		t.Fatalf("waits ran before the first wait command: %v", got)
	}

	// The next good load clears the failure.
	fail = false
	step(s4)
	st = m.Status()
	if st.Err != nil || !st.Failed.IsZero() || st.State != s4 || !st.Loaded.After(good.Loaded) {
		t.Fatalf("status %+v after the fix", st)
	}
}

func TestT18WaitArmsAgainstTheFailedState(t *testing.T) {
	s1, s2 := state("aaaaaaa1"), state("bbbbbbb2")
	w := newScript(s1)
	fail := false
	m := New(context.Background(), live, w, newStepper().now, func(_ context.Context, st watch.State) (string, error) {
		if fail {
			return "", errors.New("broken")
		}
		return "ok", nil
	})
	m, cmd := m.Update(changed{s1})
	m, _ = m.Update(run(t, cmd))
	fail = true
	m, cmd = m.Update(changed{s2})
	m, cmd = m.Update(run(t, cmd))
	w.feed <- s2
	run(t, cmd)
	// The failure leaves the wait on s2, not on the good s1: no loop.
	if got := w.armed(); len(got) != 1 || got[0] != s2 {
		t.Fatalf("armed against %v, want [%v]", got, s2)
	}
	if m.Status().State != s1 {
		t.Fatalf("state %v, want the good %v", m.Status().State, s1)
	}
}

func TestT19ErrStateSkipsTheLoader(t *testing.T) {
	calls := 0
	w := newScript(watch.State{})
	m := New(context.Background(), live, w, newStepper().now, func(context.Context, watch.State) (string, error) {
		calls++
		return "", nil
	})
	m, cmd := m.Update(changed{watch.State{Err: "not a git repository"}})
	l, ok := run(t, cmd).(Loaded[string])
	if !ok || l.Err == nil || l.Err.Error() != "not a git repository" || l.State.Err == "" {
		t.Fatalf("the load yields %#v", l)
	}
	m, _ = m.Update(l)
	if calls != 0 {
		t.Fatalf("the Loader ran %d times", calls)
	}
	if st := m.Status(); st.Good || st.Err == nil || st.Notice() != "No view yet: not a git repository" {
		t.Fatalf("status %+v", st)
	}
}

func TestT20SlowLoadDoesNotStack(t *testing.T) {
	dir := watchtest.Repo(t)
	clk := watchtest.NewClock()
	w := watch.New(watch.Source{Dir: dir})
	w.Clock = clk
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	started, release := make(chan watch.State, 4), make(chan struct{}, 4)
	m := New(ctx, w.Source, w, newStepper().now, func(_ context.Context, st watch.State) (string, error) {
		calls.Add(1)
		started <- st
		<-release
		return st.Commit, nil
	})
	async := func(cmd tea.Cmd) chan tea.Msg {
		c := make(chan tea.Msg, 1)
		go func() { c <- cmd() }()
		return c
	}

	msg := run(t, m.Init())
	m, cmd := m.Update(msg)
	loading := async(cmd)
	first := <-started

	// Two commits land during the load; nothing polls meanwhile.
	watchtest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "two")
	watchtest.Git(t, dir, "commit", "-q", "--allow-empty", "-m", "three")
	head := watchtest.Git(t, dir, "rev-parse", "HEAD")
	if n := len(clk.Asked); n != 0 {
		t.Fatalf("the watcher polled %d times during the load", n)
	}
	release <- struct{}{}
	m, cmd = m.Update(<-loading)
	if m.Status().State != first {
		t.Fatalf("state %v, want %v", m.Status().State, first)
	}

	// The wait finds both commits at once and settles once.
	waiting := async(cmd)
	if d := <-clk.Asked; d != w.Settle {
		t.Fatalf("the watcher waits %v first, want the settle %v", d, w.Settle)
	}
	clk.Tick <- clk.T
	ch, ok := (<-waiting).(changed)
	if !ok || ch.state.Commit != head {
		t.Fatalf("the wait yields %#v, want a change to %s", ch, head)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d loads before the second", calls.Load())
	}

	// One further load follows.
	m, cmd = m.Update(ch)
	loading = async(cmd)
	if second := <-started; second.Commit != head {
		t.Fatalf("the second load reads %s, want %s", second.Commit, head)
	}
	release <- struct{}{}
	m, cmd = m.Update(<-loading)
	if m.Status().State.Commit != head {
		t.Fatalf("state %v", m.Status().State)
	}

	// Then the watcher polls again, and nothing more loads.
	waiting = async(cmd)
	if d := <-clk.Asked; d != w.Interval {
		t.Fatalf("the watcher waits %v, want the interval %v", d, w.Interval)
	}
	cancel()
	if msg := <-waiting; msg != nil {
		t.Fatalf("the cancelled wait yields %#v", msg)
	}
	if calls.Load() != 2 {
		t.Fatalf("%d loads, want 2", calls.Load())
	}
}

// goldenCases reads testdata/lines.txt: case, Where and Notice per line.
func goldenCases(t *testing.T) (names []string, where, notice map[string]string) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "lines.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	where, notice = map[string]string{}, map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			t.Fatalf("line %q has %d fields, want 3", line, len(parts))
		}
		name := strings.TrimSpace(parts[0])
		names = append(names, name)
		where[name] = strings.TrimSpace(parts[1])
		notice[name] = strings.TrimSpace(parts[2])
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return names, where, notice
}

func TestT21Lines(t *testing.T) {
	const commit = "3cdae52f0123456789abcdef0123456789abcdef"
	loaded := time.Date(2026, 10, 6, 14, 2, 26, 0, time.Local)
	failed := time.Date(2026, 10, 6, 14, 3, 10, 0, time.Local)
	at := watch.Source{Dir: "/p", Ref: "v1.0"}
	main := watch.State{Commit: commit, Branch: "main"}
	pin := watch.State{Commit: commit}
	cases := map[string]Status{
		"live-first":    {Source: live},
		"live-branch":   {Source: live, Good: true, State: main, Loaded: loaded},
		"live-detached": {Source: live, Good: true, State: pin, Loaded: loaded},
		"at-first":      {Source: at},
		"at-good":       {Source: at, Good: true, State: pin, Loaded: loaded},
		"reload-failed": {Source: live, Good: true, State: main, Loaded: loaded, Failed: failed,
			Err: errors.New("status/9f31.yaml:2:8: S3 unknown state \"nomnal\"\nsecond line")},
		"repository-broken": {Source: live, Good: true, State: main, Loaded: loaded, Failed: failed,
			Err: errors.New("not a git repository (or any parent up to mount point /)")},
		"ref-gone": {Source: at, Good: true, State: pin, Loaded: loaded, Failed: failed,
			Err: errors.New("Needed a single revision")},
		"no-view-yet": {Source: live, Err: errors.New("no .tableaux directory")},
	}
	names, where, notice := goldenCases(t)
	if len(names) != len(cases) {
		t.Fatalf("testdata holds %d cases, the test builds %d", len(names), len(cases))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			st, ok := cases[name]
			if !ok {
				t.Fatalf("no status built for %q", name)
			}
			if got := st.Where(); got != where[name] {
				t.Errorf("Where %q, want %q", got, where[name])
			}
			if got := st.Notice(); got != notice[name] {
				t.Errorf("Notice %q, want %q", got, notice[name])
			}
		})
	}
}

func TestT22CancelAndForeignMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := newScript(state("aaaaaaa1"))
	calls := 0
	m := New(ctx, live, w, newStepper().now, func(context.Context, watch.State) (string, error) {
		calls++
		return "", nil
	})
	m, cmd := m.Update(changed{state("aaaaaaa1")})
	m, cmd = m.Update(run(t, cmd))
	cancel()
	// A cancelled wait, init and load each return nil, which Bubble Tea drops.
	if msg := run(t, cmd); msg != nil {
		t.Fatalf("the cancelled wait yields %#v", msg)
	}
	if msg := run(t, m.Init()); msg != nil {
		t.Fatalf("the cancelled Init yields %#v", msg)
	}
	_, cmd = m.Update(changed{state("bbbbbbb2")})
	if msg := run(t, cmd); msg != nil || calls != 1 {
		t.Fatalf("the cancelled load yields %#v after %d calls", msg, calls)
	}

	// A foreign message leaves the model unchanged and returns no command.
	before := m.Status()
	for _, foreign := range []tea.Msg{tea.KeyPressMsg{Code: 'q', Text: "q"}, tea.WindowSizeMsg{Width: 80, Height: 24}, nil, "text"} {
		got, cmd := m.Update(foreign)
		if cmd != nil || !reflect.DeepEqual(got.Status(), before) {
			t.Fatalf("%#v changed the model or returned a command", foreign)
		}
	}
	// A Loaded of another data type is foreign too.
	if got, cmd := m.Update(Loaded[int]{}); cmd != nil || !reflect.DeepEqual(got.Status(), before) {
		t.Fatal("a Loaded of another type changed the model")
	}
}
