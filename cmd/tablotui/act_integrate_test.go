package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/act"
)

// The integrate tests of 518e ("Later gates", with decision 16 a): the
// command wires the actions to tabloio's action package over the project the
// watcher watches, and T19 runs again through that wiring on a copy of the
// corpus entry weather-station, with tablo's own facts. They skip without
// TABLEAUX_CORPUS.

// planned is the Repo that main.go wires, wrapped so that a test reads the
// plan each call returns.
type planned struct {
	act.Repo
	mu   sync.Mutex
	last *action.Plan
}

func (p *planned) keep(pl *action.Plan, err error) (*action.Plan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.last = pl
	return pl, err
}

func (p *planned) plan() *action.Plan {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

func (p *planned) Record(ctx context.Context, rec action.Recording, o action.Options) (*action.Plan, error) {
	return p.keep(p.Repo.Record(ctx, rec, o))
}

func (p *planned) Review(ctx context.Context, ids []string, gate string, o action.Options) (*action.Plan, error) {
	return p.keep(p.Repo.Review(ctx, ids, gate, o))
}

func (p *planned) Reaffirm(ctx context.Context, ids []string, o action.Options) (*action.Plan, error) {
	return p.keep(p.Repo.Reaffirm(ctx, ids, o))
}

func (p *planned) Authorise(ctx context.Context, ids []string, o action.Options) (*action.Plan, error) {
	return p.keep(p.Repo.Authorise(ctx, ids, o))
}

// wired is what run hands newActions: the options as main.go builds them,
// the Repo they name, and a count of the pokes the actions make.
type wired struct {
	o     act.Options
	repo  *planned
	pokes atomic.Int32
}

// wire replaces newActions for the test, so that it keeps the options and
// counts the pokes, and changes nothing else of what the command builds.
func wire(t *testing.T) *wired {
	t.Helper()
	w := &wired{}
	old := newActions
	newActions = func(o act.Options) *act.Actions {
		w.o = o
		if o.Repo != nil {
			w.repo = &planned{Repo: o.Repo}
			o.Repo = w.repo
		}
		if poke := o.Poke; poke != nil {
			o.Poke = func() { w.pokes.Add(1); poke() }
		}
		return act.New(o)
	}
	t.Cleanup(func() { newActions = old })
	return w
}

// hand makes the person named Git's identity, the hand that commits.
func hand(t *testing.T, name, email string) {
	t.Helper()
	for _, k := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(k+"_NAME", name)
		t.Setenv(k+"_EMAIL", email)
	}
}

// git runs git in the copy of the entry.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return corpustest.Git(t, dir, args...)
}

// screenHas reports whether a line of the screen contains s.
func screenHas(s string) func([]string) bool {
	return func(l []string) bool { return strings.Contains(strings.Join(l, "\n"), s) }
}

// selectTask moves the grid's row cursor down to the task id.
func (s *session) selectTask(id string) {
	s.t.Helper()
	for range 8 {
		if s.selected().Task == id {
			return
		}
		s.key(tea.KeyPressMsg{Code: tea.KeyDown})
		n := len(s.seen)
		s.until("a selection", func([]string) bool {
			return slices.ContainsFunc(s.seen[n:], func(m tea.Msg) bool { _, ok := m.(ui.SelectionMsg); return ok })
		})
	}
	s.t.Fatalf("the grid never selects %s", id)
}

// confirmed waits for the confirmation and returns the plan it shows.
func (s *session) confirmed(w *wired) *action.Plan {
	s.t.Helper()
	s.until("the confirmation", screenHas("Commit? y commits, n cancels"))
	p := w.repo.plan()
	if p == nil {
		s.t.Fatal("the confirmation shows no plan")
	}
	return p
}

// trailersOf returns the trailers as git interpret-trailers --parse reads
// them from the message of commit.
func trailersOf(t *testing.T, dir, commit string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "interpret-trailers", "--parse")
	cmd.Stdin = strings.NewReader(git(t, dir, "log", "-1", "--format=%B", commit) + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git interpret-trailers: %v", err)
	}
	return string(out)
}

// trailerLines returns a plan's trailers as git interpret-trailers --parse
// prints them.
func trailerLines(p *action.Plan) string {
	var b strings.Builder
	for _, tr := range p.Trailers {
		b.WriteString(tr.Key + ": " + tr.Value + "\n")
	}
	return b.String()
}

// The wiring, line by line: the Repo over the watcher's project directory
// with no Source and no Env, so each plan opens its own action.Tablo (A2) and
// commits as Git's identity; the Facts that open a fresh action.Tablo for
// each form (decision 16 a); the watcher's Dir, the project above -C, and its
// Ref (A12); the viewer that identity.Email names (A14); the Context; and a
// Poke. At a --ref every action says that nothing writes there (A12, D4).
func TestIntegrateTheActionsAreWired(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	project, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	hand(t, "Ada Byron", "ada@example.org")
	for _, c := range []struct {
		args        []string
		ref, viewer string
	}{
		{[]string{"-C", filepath.Join(dir, "docs")}, "", "ada@example.org"},
		{[]string{"-C", dir, "--as", "ben@example.org"}, "", "ben@example.org"},
		{[]string{"-C", dir, "--ref", "main"}, "main", "ada@example.org"},
	} {
		w := wire(t)
		runLive(t, func(s *session) {
			s.until("the arrival", func(l []string) bool { return len(rowIDs(l)) > 0 })
			if c.ref == "" {
				return
			}
			for _, k := range "rvau" {
				s.key(text(k))
				s.until(string(k)+" at the ref", func(l []string) bool {
					return l[len(l)-2] == "--ref main shows a commit; nothing writes there"
				})
			}
		}, c.args...)
		repo, ok := w.repo.Repo.(*action.Repo)
		if !ok || repo.Source != nil || repo.Env != nil {
			t.Fatalf("%v: the Repo is %#v", c.args, w.repo.Repo)
		}
		facts, ok := w.o.Facts.(*formFacts)
		if !ok {
			t.Fatalf("%v: the Facts are %#v", c.args, w.o.Facts)
		}
		if repo.Dir != project || facts.dir != project || w.o.Dir != project {
			t.Errorf("%v: the directories %q, %q and %q, want %q", c.args, repo.Dir, facts.dir, w.o.Dir, project)
		}
		if w.o.Ref != c.ref || w.o.Viewer != c.viewer || w.o.Poke == nil || w.o.Context == nil {
			t.Errorf("%v: Ref %q, Viewer %q, Poke set %v, Context set %v", c.args, w.o.Ref, w.o.Viewer, w.o.Poke != nil, w.o.Context != nil)
		}
	}
}

// T19 on weather-station, through main.go's wiring and tablo's facts: Ben
// records 9f31 at release, which carries his review of validate; reaffirms
// it; and meets a pre-commit hook that fails. Each commit equals its plan,
// and after each Apply the watcher's poke reloads, so the grid shows the new
// commit with no key pressed. The last form reads the status the first
// commit wrote, so each form has a fresh action.Tablo (decision 16 a).
func TestIntegrateTheActionsCommit(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	hand(t, "Ben Okafor", "ben@example.org")
	w := wire(t)
	runLive(t, func(s *session) {
		s.until("Ben's corner", func(l []string) bool {
			return strings.HasSuffix(l[0], "ben@example.org, assignee") && len(rowIDs(l)) > 0 && s.selected().Task != ""
		})
		s.selectTask("9f31")

		// r: function to release, four choices to the left, round the end.
		head := git(t, dir, "rev-parse", "HEAD")
		s.key(text('r'))
		s.until("the form", screenHas("  now     function stalled, blocked · Barometer ICs on 14-week backorder"))
		for range 4 {
			s.key(tea.KeyPressMsg{Code: tea.KeyLeft})
		}
		s.key(tea.KeyPressMsg{Code: tea.KeyEnter})
		p := s.confirmed(w)
		if p.Base != head || trailerLines(p) != "Reviewed: 9f31 validate\n" {
			t.Errorf("the plan at %s carries %q", p.Base, trailerLines(p))
		}
		s.key(text('y'))
		s.until("the new commit in the grid", func(l []string) bool {
			c := git(t, dir, "rev-parse", "--short=7", "HEAD")
			return strings.Contains(l[0], "main at "+c+",") && screenHas("9f31 Sensor board · 🚀 release ✅ complete")(l)
		})
		commit := git(t, dir, "rev-parse", "HEAD")
		if git(t, dir, "rev-parse", "HEAD~1") != head || git(t, dir, "log", "-1", "--format=%s") != p.Subject {
			t.Errorf("the commit %s does not follow %s with %q", commit, head, p.Subject)
		}
		if got, want := trailersOf(t, dir, commit), trailerLines(p); got != want {
			t.Errorf("the commit's trailers %q, the plan's %q", got, want)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".tableaux", "status", "9f31.yaml")); string(b) != "# status/9f31.yaml : as in SYNTAX.md\ngate: release\nstate: complete\n" {
			t.Errorf("the status file holds %q", b)
		}
		if n := w.pokes.Load(); n != 1 {
			t.Errorf("%d pokes after the record", n)
		}

		// a: the empty Reaffirmed: commit.
		head = commit
		s.key(text('a'))
		p = s.confirmed(w)
		if p.Base != head || trailerLines(p) != "Reaffirmed: 9f31\n" {
			t.Errorf("the reaffirmation at %s carries %q", p.Base, trailerLines(p))
		}
		s.key(text('y'))
		s.until("the reaffirmation in the grid", func(l []string) bool {
			c := git(t, dir, "rev-parse", "--short=7", "HEAD")
			return c != head[:7] && strings.Contains(l[0], "main at "+c+",")
		})
		commit = git(t, dir, "rev-parse", "HEAD")
		if git(t, dir, "rev-parse", "HEAD~1") != head || git(t, dir, "diff", "--name-only", head, commit) != "" {
			t.Errorf("the reaffirmation %s is not an empty commit on %s", commit, head)
		}
		if got, want := trailersOf(t, dir, commit), trailerLines(p); got != want || git(t, dir, "log", "-1", "--format=%s") != "Reaffirm sensor board (9f31)" {
			t.Errorf("the reaffirmation's trailers %q, the plan's %q", got, want)
		}
		if n := w.pokes.Load(); n != 2 {
			t.Errorf("%d pokes after the reaffirmation", n)
		}

		// A failing pre-commit hook: not committed, a clean tree, HEAD where
		// it stood. The form reads the HEAD of the reaffirmation.
		hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
		if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'pre-commit: tablo validate reports 1 error' >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		head = commit
		var got []string
		s.key(text('r'))
		s.until("the form at the new HEAD", screenHas("  now     release complete\n"))
		s.key(tea.KeyPressMsg{Code: tea.KeyUp})
		for _, r := range "Hook" {
			s.key(text(r))
		}
		s.key(tea.KeyPressMsg{Code: tea.KeyEnter})
		if p = s.confirmed(w); p.Base != head {
			t.Errorf("the plan reads %s, HEAD is %s", p.Base, head)
		}
		s.key(text('y'))
		failed := tea.Msg(ui.NoticeMsg{Text: "not committed: git commit: exit 1", Err: true})
		got = s.until("the hook's failure", func(l []string) bool {
			return l[1] == "record 9f31: not committed" && slices.Contains(s.seen, failed) && w.pokes.Load() == 3
		})
		if j := strings.Join(got, "\n"); !strings.Contains(j, "pre-commit: tablo validate reports 1 error") ||
			!strings.Contains(j, "the checkout stands as it stood before the commit") {
			t.Errorf("the hook's failure:\n%s", j)
		}
		if st := git(t, dir, "status", "--porcelain"); st != "" {
			t.Errorf("git status --porcelain: %q", st)
		}
		if now := git(t, dir, "rev-parse", "HEAD"); now != head {
			t.Errorf("HEAD moved to %s", now)
		}
	}, "-C", dir, "--as", "ben@example.org")
}

// gotos returns the ui.GotoMsg the panes sent, in order.
func (s *session) gotos() []ui.GotoMsg {
	var out []ui.GotoMsg
	for _, m := range s.seen {
		if g, ok := m.(ui.GotoMsg); ok {
			out = append(out, g)
		}
	}
	return out
}

// A13 (0afa, as decision 14 narrows it): no pane binds r, v, a or u, since
// run hands every pane's keys and the four commands to ui.CheckKeys, and
// every run here exits 0; the frame routes a command before the focused pane,
// so a works from Ada's queue; and enter on the queue's reaffirmation owed,
// 9f31 at function, moves the grid's selection through ui.GotoMsg, which the
// actions then act on.
func TestIntegrateTheActionsFollowTheQueue(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	hand(t, "Ada Byron", "ada@example.org")
	w := wire(t)
	runLive(t, func(s *session) {
		s.until("Ada's arrival", func(l []string) bool {
			return strings.HasSuffix(l[0], "ada@example.org, owner") && len(rowIDs(l)) > 0 && s.selected().Task == "a1c0"
		})
		s.update(tea.WindowSizeMsg{Width: 160, Height: 45})
		s.key(text('2'))
		s.until("Ada's queue", func(l []string) bool {
			return screenHas("Reaffirmations: 1")(l) && strings.HasPrefix(l[len(l)-2], "queue ada@example.org · ")
		})
		for i := 0; ; i++ {
			if g := s.gotos(); len(g) > 0 && g[len(g)-1].Task == "9f31" {
				if want := (ui.GotoMsg{Task: "9f31", Gate: "function"}); g[len(g)-1] != want {
					t.Errorf("the queue's first line of 9f31 sends %+v, want %+v", g[len(g)-1], want)
				}
				break
			}
			if i == 14 {
				t.Fatalf("no line of the queue names 9f31: %+v", s.gotos())
			}
			s.key(tea.KeyPressMsg{Code: tea.KeyDown})
			s.key(tea.KeyPressMsg{Code: tea.KeyEnter})
			n := len(s.seen)
			s.until("an answer to enter", func(l []string) bool {
				return len(s.seen) > n || l[len(l)-2] == "this line names no task"
			})
		}
		s.until("the grid on 9f31", func([]string) bool { return s.selected().Task == "9f31" })
		s.key(text('a'))
		p := s.confirmed(w)
		if p.Command != "reaffirm" || !slices.Equal(p.Tasks, []string{"9f31"}) || len(p.Warnings) != 0 {
			t.Errorf("a from the queue plans %s %v with %v", p.Command, p.Tasks, p.Warnings)
		}
		s.key(text('n'))
		s.until("the cancel", func(l []string) bool { return l[len(l)-2] == "not committed" })
	}, "-C", dir, "--as", "ada@example.org")
}

// A14 (f394) and f394's A12, which its decision 14 (b) moves here: --as
// names the viewer, whom the actions take from identity.Email as
// Options.Viewer, and never the identity Git commits with. Ada's Git commits
// a reaffirmation that Ben's view asks for: the confirmation says the
// commit is Ada's, and the commit is. Without --as the viewer is Ada's own
// email, and the confirmation says nothing of it.
func TestIntegrateTheActionsCommitAsGit(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	hand(t, "Ada Byron", "ada@example.org")
	const drawn = "drawn for ben@example.org; the commit is ada@example.org's"
	for _, c := range []struct {
		as, viewer string
		drawn      bool
	}{
		{"ben@example.org", "ben@example.org", true},
		{"", "ada@example.org", false},
	} {
		w := wire(t)
		args := []string{"-C", dir}
		if c.as != "" {
			args = append(args, "--as", c.as)
		}
		runLive(t, func(s *session) {
			s.until("the arrival", func(l []string) bool {
				return strings.Contains(l[0], "viewer "+c.viewer+", ") && len(rowIDs(l)) > 0 && s.selected().Task != ""
			})
			if !slices.Contains(rowIDs(lines(s.m)), "9f31") {
				// The owner's tableau folds 4e2b; enter on it unfolds it.
				s.selectTask("4e2b")
				s.key(tea.KeyPressMsg{Code: tea.KeyEnter})
				s.until("4e2b unfolded", func(l []string) bool { return slices.Contains(rowIDs(l), "9f31") })
			}
			s.selectTask("9f31")
			head := git(t, dir, "rev-parse", "HEAD")
			s.key(text('a'))
			s.confirmed(w)
			if got := screenHas(drawn)(lines(s.m)); got != c.drawn {
				t.Errorf("--as %q: the confirmation states %q: %v\n%s", c.as, drawn, got, strings.Join(lines(s.m), "\n"))
			}
			s.key(text('y'))
			s.until("the commit", func([]string) bool { return git(t, dir, "rev-parse", "HEAD~1") == head })
			if got := git(t, dir, "log", "-1", "--format=%an <%ae> / %cn <%ce>"); got != "Ada Byron <ada@example.org> / Ada Byron <ada@example.org>" {
				t.Errorf("--as %q: the commit is %s's", c.as, got)
			}
		}, args...)
		if w.o.Viewer != c.viewer {
			t.Errorf("--as %q: Viewer %q, want %q", c.as, w.o.Viewer, c.viewer)
		}
	}
}
