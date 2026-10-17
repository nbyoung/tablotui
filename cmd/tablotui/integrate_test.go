package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
)

// The integrate tests of f394 ("Later gates", with decision 13): the command
// on a copy of the corpus entry weather-station, through tablo's library and
// the adapter that implements source.Viewers (A9). Ada arrives as the owner
// and reaches her corner with R; Ben arrives on his corner with a1c0 and
// 4e2b unfolded and 4e2b selected; an email outside the project reads as an
// observer; the role pane's ref follows --ref (A10); the role pane sits with
// 0afa's panes and tells them whom they read for (A11); and the command binds
// R alone of the keys f394 and 518e name, none of r, v, a and u, which
// 518e's own integrate gate binds (A12). They skip without TABLEAUX_CORPUS.

// session drives the model as the program does, each command in its own
// goroutine, and records every message the commands deliver. The watcher's
// wait blocks until the program's context ends.
type session struct {
	t    *testing.T
	m    tea.Model
	msgs chan tea.Msg
	seen []tea.Msg
}

func newSession(t *testing.T, m tea.Model, w, h int) *session {
	t.Helper()
	s := &session{t: t, m: m, msgs: make(chan tea.Msg, 1024)}
	s.update(tea.WindowSizeMsg{Width: w, Height: h})
	s.start(s.m.Init())
	return s
}

func (s *session) start(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		switch msg := cmd().(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range msg {
				s.start(c)
			}
		default:
			s.msgs <- msg
		}
	}()
}

func (s *session) update(msg tea.Msg) {
	var cmd tea.Cmd
	s.m, cmd = s.m.Update(msg)
	s.start(cmd)
}

// until delivers messages until done accepts the stripped screen.
func (s *session) until(what string, done func([]string) bool) []string {
	s.t.Helper()
	if got := lines(s.m); done(got) {
		return got
	}
	timeout := time.After(time.Minute)
	for {
		select {
		case msg := <-s.msgs:
			s.seen = append(s.seen, msg)
			s.update(msg)
			if got := lines(s.m); done(got) {
				return got
			}
		case <-timeout:
			s.t.Fatalf("the screen never shows %s:\n%s", what, strings.Join(lines(s.m), "\n"))
		}
	}
}

// key presses one key, named by its text or as tea names it.
func (s *session) key(k tea.KeyPressMsg) { s.update(k) }

func text(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

// starts returns the grid.StartMsg the role pane sent, in order.
func (s *session) starts() []grid.Start {
	var out []grid.Start
	for _, m := range s.seen {
		if st, ok := m.(grid.StartMsg); ok {
			out = append(out, st.Start)
		}
	}
	return out
}

// selected returns the newest ui.SelectionMsg the grid sent.
func (s *session) selected() ui.SelectionMsg {
	var sel ui.SelectionMsg
	for _, m := range s.seen {
		if v, ok := m.(ui.SelectionMsg); ok {
			sel = v
		}
	}
	return sel
}

// runLive runs the command with args on its own program and hands the model
// to f while the program would run.
func runLive(t *testing.T, f func(s *session), args ...string) {
	t.Helper()
	h := newHarness(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h.onRun = func(m tea.Model) { f(newSession(t, m, 120, 24)) }
	if code, _, errOut := runArgs(args...); code != 0 || errOut != "" || h.calls != 1 {
		t.Fatalf("exit %d, stderr %q, calls %d", code, errOut, h.calls)
	}
}

// rowIDs returns the ids that begin the body's rows, from the line after the
// header's rule to the status line.
func rowIDs(screen []string) []string {
	var ids []string
	for _, l := range screen[3 : len(screen)-2] {
		if id, _, ok := strings.Cut(l, " "); ok && len(id) == 4 {
			ids = append(ids, id)
		}
	}
	return ids
}

// Ada holds every role but the reviewer's: she arrives as the owner on the
// global tableau at a glance, the root and its children, and R takes her to
// her corner, the person form at detail, as the assignee.
func TestIntegrateAdaReachesHerCorner(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	runLive(t, func(s *session) {
		got := s.until("Ada's arrival", func(l []string) bool {
			return strings.HasSuffix(l[0], "viewer ada@example.org, owner") && len(rowIDs(l)) > 0
		})
		if ids := rowIDs(got); !slices.Equal(ids, []string{"a1c0", "4e2b", "7b2e", "3c5d"}) {
			t.Errorf("the owner's arrival draws %v:\n%s", ids, strings.Join(got, "\n"))
		}
		s.key(text('R'))
		got = s.until("Ada's corner", func(l []string) bool {
			return strings.HasSuffix(l[0], "viewer ada@example.org, assignee") && l[len(l)-2] == "reading as assignee, 2 of 2"
		})
		st := s.starts()
		if last := st[len(st)-1]; last.Request.View != "context" || last.Request.Person != "ada@example.org" || last.Level != grid.Detail || !last.Mine {
			t.Errorf("the corner's start %+v", last)
		}
		if ids := rowIDs(got); len(ids) == 0 || ids[0] != "a1c0" {
			t.Errorf("the corner draws %v:\n%s", ids, strings.Join(got, "\n"))
		}
	}, "-C", dir, "--as", "ada@example.org")
}

// Decision 13: Ben arrives on his corner, a1c0 and 4e2b unfolded and 4e2b,
// his own task, selected.
func TestIntegrateBenArrivesOnHisCorner(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	runLive(t, func(s *session) {
		got := s.until("Ben's corner", func(l []string) bool {
			return strings.HasSuffix(l[0], "viewer ben@example.org, assignee") && len(rowIDs(l)) > 0 && s.selected().Task != ""
		})
		ids := rowIDs(got)
		if !slices.Contains(ids, "4e2b") || !slices.Contains(ids, "9f31") || !slices.Contains(ids, "c07d") {
			t.Errorf("a1c0 and 4e2b do not stand unfolded: %v\n%s", ids, strings.Join(got, "\n"))
		}
		if got := s.selected().Task; got != "4e2b" {
			t.Errorf("the arrival selects %s, want 4e2b", got)
		}
	}, "-C", dir, "--as", "ben@example.org")
}

// An email outside the project reads as an observer: the global tableau at a
// glance, and R names its one role.
func TestIntegrateAStrangerReadsAsAnObserver(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	runLive(t, func(s *session) {
		got := s.until("the observer", func(l []string) bool {
			return strings.HasSuffix(l[0], "viewer eve@example.org, observer") && len(rowIDs(l)) > 0
		})
		if ids := rowIDs(got); !slices.Equal(ids, []string{"a1c0", "4e2b", "7b2e", "3c5d"}) {
			t.Errorf("the observer's arrival draws %v", ids)
		}
		s.key(text('R'))
		s.until("the one role", func(l []string) bool { return l[len(l)-2] == "one role only: observer" })
	}, "-C", dir, "--as", "eve@example.org")
}

// A10: the role pane's ref is --ref: every start it sends names it, and the
// context line names the ref and its commit.
func TestIntegrateTheRoleFollowsTheRef(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	labels := corpustest.Labels(t, "weather-station")
	for _, ref := range []string{"main", labels["W12"]} {
		t.Run(ref[:4], func(t *testing.T) {
			runLive(t, func(s *session) {
				s.until("the arrival at "+ref, func(l []string) bool {
					return strings.HasPrefix(l[0], "Weather station · "+ref+" at ") && strings.HasSuffix(l[0], "ben@example.org, assignee") && len(rowIDs(l)) > 0
				})
				s.key(text('R'))
				s.until("the reviewer", func(l []string) bool { return strings.HasSuffix(l[0], "ben@example.org, reviewer") })
				st := s.starts()
				if len(st) < 2 {
					t.Fatalf("starts %+v", st)
				}
				for _, start := range st {
					if start.Request.Ref != ref {
						t.Errorf("a start names the ref %q, want %q", start.Request.Ref, ref)
					}
				}
			}, "-C", dir, "--ref", ref, "--as", "ben@example.org")
		})
	}
}

// A11: the role pane, last of the panes, never draws; the four panes of 0afa
// open beside the grid, read for the viewer the role pane names, and after R
// ask again as the new role, at the level tablo opens each view at for it.
func TestIntegrateTheRoleSitsWithThePanes(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	runLive(t, func(s *session) {
		s.until("Ben's corner", func(l []string) bool {
			return strings.HasSuffix(l[0], "ben@example.org, assignee") && len(rowIDs(l)) > 0
		})
		s.update(tea.WindowSizeMsg{Width: 160, Height: 45})
		for _, k := range "1234" {
			s.key(text(k))
		}
		got := s.until("the four panes", func(l []string) bool {
			j := strings.Join(l, "\n")
			return strings.Contains(j, "2 Queue ben@example.org · ") && strings.Contains(j, "4 History 4e2b · ") && !strings.Contains(j, "loading…")
		})
		screen := strings.Join(got, "\n")
		if !strings.Contains(screen, "· glance ─") || strings.Contains(screen, "role") {
			t.Errorf("the queue opens at glance for the assignee, and no pane names the role pane:\n%s", screen)
		}
		s.key(text('R'))
		got = s.until("the reviewer's panes", func(l []string) bool {
			j := strings.Join(l, "\n")
			return strings.HasSuffix(l[0], "ben@example.org, reviewer") && strings.Contains(j, "2 Queue ben@example.org · ") && !strings.Contains(j, "loading…")
		})
		for _, l := range got {
			if strings.Contains(l, "2 Queue ben@example.org") && !strings.Contains(l, " · detail ") {
				t.Errorf("the reviewer's queue does not open at detail: %q", l)
			}
		}
	}, "-C", dir, "--as", "ben@example.org")
}

// A12, as far as this task binds it: the command binds R, which the help
// mode lists as the role, and none of r, v, a and u, which 518e's commands
// take at its own integrate gate.
func TestIntegrateRAloneIsBound(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	runLive(t, func(s *session) {
		s.until("Ben's corner", func(l []string) bool {
			return strings.HasSuffix(l[0], "ben@example.org, assignee") && len(rowIDs(l)) > 0
		})
		s.update(tea.WindowSizeMsg{Width: 120, Height: 45})
		s.key(text('?'))
		bound := func(l []string) (keys []string) {
			for _, line := range l {
				if f := strings.Fields(line); len(f) >= 2 {
					keys = append(keys, f[0])
				}
			}
			return keys
		}
		got := s.until("the help mode", func(l []string) bool {
			return slices.ContainsFunc(l, func(line string) bool { return slices.Equal(strings.Fields(line), []string{"R", "role"}) })
		})
		keys := bound(got[1 : len(got)-2])
		if !slices.Contains(keys, "R") || !slices.Contains(keys, "1") {
			t.Errorf("the help mode lists %v", keys)
		}
		for _, k := range keys {
			if slices.Contains([]string{"r", "v", "a", "u"}, k) {
				t.Errorf("the help mode binds %s", k)
			}
		}
		s.key(tea.KeyPressMsg{Code: tea.KeyEscape})
		before := strings.Join(lines(s.m), "\n")
		for _, k := range "rvau" {
			s.key(text(k))
		}
		if after := strings.Join(lines(s.m), "\n"); after != before {
			t.Errorf("r, v, a and u change the screen:\n%s\nwas:\n%s", after, before)
		}
	}, "-C", dir, "--as", "ben@example.org")
}

// The integrate tests of 171b that decision 11 keeps for this gate, through
// the command on a copy of weather-station: Where at the end of the status
// line, the notice line of the live refresh's own, and the Force key.

// reloads counts the ui.ReloadMsg the session has seen.
func (s *session) reloads() int {
	n := 0
	for _, m := range s.seen {
		if _, ok := m.(ui.ReloadMsg); ok {
			n++
		}
	}
	return n
}

var ctrlR = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}

// A5 and A6: the status line ends with the source, the working tree on its
// branch at its commit; ctrl+r reloads at once; a file that breaks the
// project keeps the view and states the failure on a line of its own above
// the status line, and the fix clears it.
func TestIntegrateTheLiveRefreshReports(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	labels := corpustest.Labels(t, "weather-station")
	at := labels["W13"][:7]
	version := filepath.Join(dir, ".tableaux", "version.yaml")
	body, err := os.ReadFile(version)
	if err != nil {
		t.Fatal(err)
	}
	runLive(t, func(s *session) {
		got := s.until("the arrival", func(l []string) bool { return len(rowIDs(l)) > 0 && s.reloads() > 0 })
		if status := got[len(got)-2]; !strings.HasSuffix(status, " · working tree on main at "+at) {
			t.Errorf("status line %q", status)
		}
		n := s.reloads()
		s.key(ctrlR)
		s.until("a forced reload", func([]string) bool { return s.reloads() > n })

		if err := os.Remove(version); err != nil {
			t.Fatal(err)
		}
		s.key(ctrlR)
		got = s.until("the failure", func(l []string) bool { return strings.HasPrefix(l[len(l)-3], "Reload failed at ") })
		if notice := got[len(got)-3]; !strings.Contains(notice, ", showing the view of ") || !strings.Contains(notice, "P2 ") {
			t.Errorf("notice line %q", notice)
		}
		if ids := rowIDs(got); !slices.Contains(ids, "a1c0") {
			t.Errorf("the view does not stand beside the failure:\n%s", strings.Join(got, "\n"))
		}

		if err := os.WriteFile(version, body, 0o644); err != nil {
			t.Fatal(err)
		}
		s.key(ctrlR)
		got = s.until("the fix", func(l []string) bool {
			return !strings.HasPrefix(l[len(l)-3], "Reload failed") && strings.HasSuffix(l[len(l)-2], "working tree on main at "+at)
		})
		if ids := rowIDs(got); !slices.Contains(ids, "a1c0") {
			t.Errorf("the view does not stand after the fix:\n%s", strings.Join(got, "\n"))
		}
	}, "-C", dir, "--as", "ada@example.org")
}

// With --ref the status line names the ref and its commit, and the help mode
// lists the Force key; with --fixture no Force key is bound.
func TestIntegrateTheRefInTheStatusLine(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	labels := corpustest.Labels(t, "weather-station")
	runLive(t, func(s *session) {
		got := s.until("the arrival", func(l []string) bool { return len(rowIDs(l)) > 0 })
		if status := got[len(got)-2]; !strings.HasSuffix(status, " · main at "+labels["W13"][:7]) {
			t.Errorf("status line %q", status)
		}
		s.update(tea.WindowSizeMsg{Width: 120, Height: 45})
		s.key(text('?'))
		s.until("the Force key in the help mode", func(l []string) bool {
			return slices.ContainsFunc(l, func(line string) bool { return slices.Equal(strings.Fields(line), []string{"ctrl+r", "reload"}) })
		})
	}, "-C", dir, "--ref", "main", "--as", "ada@example.org")

	h := newHarness(t)
	var help []string
	h.onRun = func(m tea.Model) { help = lines(press(drive(m, 100, 40), text('?'))) }
	if code, _, errOut := runArgs("--fixture", fixtureDir(t)); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if j := strings.Join(help, "\n"); strings.Contains(j, "ctrl+r") || !strings.Contains(j, "Commands") {
		t.Errorf("with --fixture the help mode reads:\n%s", j)
	}
}
