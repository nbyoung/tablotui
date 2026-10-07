package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestMain gives every test of the command no Git identity: no author
// variable, no global or system configuration, and a working directory that
// no repository holds, so that the host's identity never reaches a test.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	for _, kv := range [][2]string{{"GIT_CONFIG_GLOBAL", os.DevNull}, {"GIT_CONFIG_SYSTEM", os.DevNull}, {"GIT_AUTHOR_EMAIL", ""}} {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			panic(err)
		}
	}
	dir, err := os.MkdirTemp("", "tablotui-main")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	return m.Run()
}

func TestResolveVersion(t *testing.T) {
	installed := &debug.BuildInfo{Main: debug.Module{Version: "v0.3.1"}}
	devel := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	cases := []struct {
		name   string
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"linker wins", "v1.2.3", installed, "v1.2.3"},
		{"module version when not linked", "dev", installed, "v0.3.1"},
		{"devel falls back", "dev", devel, "dev"},
		{"no build info falls back", "dev", nil, "dev"},
		{"empty linked reads build info", "", installed, "v0.3.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveVersion(c.linked, c.info); got != c.want {
				t.Errorf("resolveVersion(%q, %v) = %q, want %q", c.linked, c.info, got, c.want)
			}
		})
	}
}

func TestBanner(t *testing.T) {
	if got, want := banner("v1.0.0"), "tablotui v1.0.0"; got != want {
		t.Errorf("banner = %q, want %q", got, want)
	}
}

const tableau = `{"view":"tableau","project":"Fixture project","ref":"main","commit":"abc1234","date":"2026-10-06",
"window":1,"gates":[{"key":"defined","symbol":"📝","name":"Defined","window":true,"count":1},
{"key":"design","symbol":"📐","name":"Design","window":true,"count":0}],
"rows":[{"id":"aaaa","title":"Only task","depth":0,"parent":false,"status":{"gate":"defined","state":"nominal"},
"cells":[{"kind":"status","symbols":"🟢"},{"kind":"marks","symbols":"🤖"}]}]}`

// harness replaces the program with one that hands the model to onRun while
// the program would run, as the context is live only then.
type harness struct {
	t     *testing.T
	onRun func(m tea.Model)
	err   error
	calls int
	ctx   context.Context
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t}
	old := runProgram
	runProgram = func(ctx context.Context, m tea.Model) error {
		h.calls++
		h.ctx = ctx
		if h.onRun != nil {
			h.onRun(m)
		}
		return h.err
	}
	t.Cleanup(func() { runProgram = old })
	return h
}

// drive sizes the model, runs its first load and returns the model.
func drive(m tea.Model, w, h int) tea.Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	var run func(cmd tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range msg {
				run(c)
			}
		default:
			var next tea.Cmd
			m, next = m.Update(msg)
			run(next)
		}
	}
	run(m.Init())
	return m
}

// lines returns the drawn lines of m, stripped.
func lines(m tea.Model) []string {
	out := strings.Split(m.View().Content, "\n")
	for i, l := range out {
		out[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return out
}

func press(m tea.Model, keys ...tea.KeyPressMsg) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(k)
	}
	return m
}

func fixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tableau.json"), []byte(tableau), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runArgs(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// T16: the command.
func TestVersionOption(t *testing.T) {
	h := newHarness(t)
	code, out, _ := runArgs("--version")
	info, _ := debug.ReadBuildInfo()
	if code != 0 || out != banner(resolveVersion(version, info))+"\n" {
		t.Errorf("--version: %d %q", code, out)
	}
	if h.calls != 0 {
		t.Error("--version started the program")
	}
}

func TestHelpOption(t *testing.T) {
	h := newHarness(t)
	code, out, _ := runArgs("--help")
	if code != 0 {
		t.Errorf("--help exits %d", code)
	}
	for _, want := range []string{"usage: tablotui", "--as", "--settings", "--fixture", "--version", "-C"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage lacks %q:\n%s", want, out)
		}
	}
	if h.calls != 0 {
		t.Error("--help started the program")
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"single hyphen", []string{"-fixture", "x"}, "--fixture"},
		{"single hyphen as", []string{"-as", "ada@example.org", "--fixture", "x"}, "--as"},
		{"as is no email", []string{"--as", "ada", "--fixture", "x"}, "usage: --as ada: not an email"},
		{"as is no email, joined", []string{"--as=ada@", "--fixture", "x"}, "not an email"},
		{"single hyphen with a value", []string{"-settings=x"}, "--settings"},
		{"single hyphen version", []string{"-version"}, "--version"},
		{"unknown option", []string{"--nope"}, "nope"},
		{"unknown short option", []string{"-x"}, "-x"},
		{"a positional argument", []string{"extra"}, "extra"},
		{"a missing value", []string{"--fixture"}, "fixture"},
		{"-C names no directory", []string{"-C", filepath.Join(t.TempDir(), "none"), "--fixture", "x"}, "-C"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, _, errOut := runArgs(c.args...)
			if code != 2 || !strings.Contains(errOut, c.want) {
				t.Errorf("exit %d, stderr %q, want exit 2 naming %q", code, errOut, c.want)
			}
		})
	}
	if h.calls != 0 {
		t.Error("a usage error started the program")
	}
}

func TestNoSourceExitsOne(t *testing.T) {
	h := newHarness(t)
	code, out, errOut := runArgs()
	if code != 1 || out != "" || errOut != "tablotui: this build has no tablo source; pass --fixture <dir>\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if h.calls != 0 {
		t.Error("the program started without a source")
	}
}

func TestFixtureDrawsTheGrid(t *testing.T) {
	h := newHarness(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var got []string
	h.onRun = func(m tea.Model) { got = lines(drive(m, 60, 12)) }
	code, _, errOut := runArgs("--fixture", fixtureDir(t))
	if code != 0 || errOut != "" || h.calls != 1 {
		t.Fatalf("exit %d, stderr %q, calls %d", code, errOut, h.calls)
	}
	if got[0] != "Fixture project · main at abc1234, 2026-10-06" || !strings.HasPrefix(got[1], "Id") || !strings.HasPrefix(got[3], "aaaa") {
		t.Errorf("screen:\n%s", strings.Join(got, "\n"))
	}
	if err := h.ctx.Err(); err == nil {
		t.Error("the context lives on after the program ends")
	}
}

// typed sends keys and runs the commands they start, as the program does.
func typed(m tea.Model, keys ...tea.KeyPressMsg) tea.Model {
	var run func(cmd tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range msg {
				run(c)
			}
		default:
			var next tea.Cmd
			m, next = m.Update(msg)
			run(next)
		}
	}
	for _, k := range keys {
		var cmd tea.Cmd
		m, cmd = m.Update(k)
		run(cmd)
	}
	return m
}

// T18: the command hands the grid, the four panes, their five commands and
// their layout to the frame, and no key clashes.
func TestFixtureHandsTheFrameFivePanesAndFiveCommands(t *testing.T) {
	h := newHarness(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	key := func(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }
	var arrival, opened, zoomed, again []string
	h.onRun = func(m tea.Model) {
		m = drive(m, 100, 30)
		arrival = lines(m)
		m = typed(m, key('1'), key('2'), key('3'), key('4'))
		opened = lines(m)
		m = typed(m, key('z'))
		zoomed = lines(m)
		again = lines(typed(m, key('z')))
	}
	dir := fixtureDir(t)
	for name, body := range map[string]string{
		"task-aaaa.json":     `{"level":"detail","task":{"id":"aaaa","title":"Only task"},"assignee":"ada@example.org"}`,
		"queue.json":         `{"level":"detail","params":{"person":"ada@example.org"}}`,
		"blockage-aaaa.json": `{"level":"detail","params":{"task":"aaaa"}}`,
		"history-aaaa.json":  `{"level":"detail","params":{"task":"aaaa"}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, _, errOut := runArgs("--fixture", dir)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q: CheckKeys refused the keys", code, errOut)
	}
	if strings.Contains(strings.Join(arrival, "\n"), "┌─") {
		t.Error("a pane stands open on arrival")
	}
	// With four panes open at 100x30 the region beneath the grid shows them
	// in two rows of two.
	screen := strings.Join(opened, "\n")
	for _, want := range []string{"1 Task aaaa · detail", "2 Queue ada@example.org · 0 items", "3 Blockage aaaa · 0 causes", "4 History aaaa · 0 events"} {
		if !strings.Contains(screen, want) {
			t.Errorf("%q is not on the screen:\n%s", want, screen)
		}
	}
	// z gives the focused pane, the history, the whole body; z again returns.
	if z := strings.Join(zoomed, "\n"); strings.Contains(z, "Only task") || !strings.Contains(zoomed[1], "┌─ 4 History aaaa") {
		t.Errorf("zoom:\n%s", z)
	}
	if a := strings.Join(again, "\n"); !strings.Contains(a, "Only task") || !strings.Contains(a, "┌─ 3 Blockage") {
		t.Errorf("zoom off:\n%s", a)
	}
}

func TestCDirResolvesRelativePaths(t *testing.T) {
	h := newHarness(t)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "fx"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "fx", "tableau.json"), []byte(tableau), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "mine.json"), []byte(`{"version":1,"columns":{"design":"hide"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	h.onRun = func(m tea.Model) { got = lines(drive(m, 60, 12)) }
	code, _, errOut := runArgs("-C", project, "--fixture", "fx", "--settings", "mine.json")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(got[1], "📐 ×0") {
		t.Errorf("--settings was not read: %q", got[1])
	}
}

func TestSettingsOptionNamesTheFile(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	h.onRun = func(m tea.Model) {
		press(drive(m, 60, 12), tea.KeyPressMsg{Code: tea.KeyRight}, tea.KeyPressMsg{Code: tea.KeyRight}, tea.KeyPressMsg{Code: 'x', Text: "x"})
	}
	if code, _, errOut := runArgs("--fixture", fixtureDir(t), "--settings", path); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), `"design": "hide"`) {
		t.Errorf("the settings file = %q, %v", b, err)
	}
}

func TestDefaultSettingsPath(t *testing.T) {
	h := newHarness(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(xdg, "tablotui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"version":1,"columns":{"design":"hide"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	h.onRun = func(m tea.Model) { got = lines(drive(m, 60, 12)) }
	if code, _, errOut := runArgs("--fixture", fixtureDir(t)); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(got[1], "📐 ×0") {
		t.Errorf("the default settings file was not read: %q", got[1])
	}
}

func TestProgramFailureExitsOne(t *testing.T) {
	h := newHarness(t)
	h.err = errors.New("no terminal")
	code, _, errOut := runArgs("--fixture", fixtureDir(t))
	if code != 1 || errOut != "tablotui: no terminal\n" {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestALoadThatFailsNeverEndsTheProgram(t *testing.T) {
	h := newHarness(t)
	var got []string
	h.onRun = func(m tea.Model) { got = lines(drive(m, 70, 12)) }
	code, _, _ := runArgs("--fixture", filepath.Join(t.TempDir(), "missing"))
	if code != 0 || h.calls != 1 {
		t.Fatalf("exit %d, calls %d", code, h.calls)
	}
	if !strings.HasPrefix(got[1], "error: ") {
		t.Errorf("body = %q", got[1])
	}
}

// T16: --as and the Git identity reach the context line.
func TestTheViewerReachesTheContextLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cases := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"--as", "", []string{"--as", "ada@example.org"}, "Fixture project · main at abc1234, 2026-10-06 · viewer ada@example.org, observer"},
		{"the author variable", "ben@example.org", nil, "Fixture project · main at abc1234, 2026-10-06 · viewer ben@example.org, observer"},
		{"--as before the variable", "ben@example.org", []string{"--as", "ada@example.org"}, "Fixture project · main at abc1234, 2026-10-06 · viewer ada@example.org, observer"},
		{"nobody", "", nil, "Fixture project · main at abc1234, 2026-10-06"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GIT_AUTHOR_EMAIL", c.env)
			var got []string
			h.onRun = func(m tea.Model) { got = lines(drive(m, 100, 12)) }
			code, _, errOut := runArgs(append([]string{"--fixture", fixtureDir(t)}, c.args...)...)
			if code != 0 || errOut != "" || h.calls != 1 {
				t.Fatalf("exit %d, stderr %q, calls %d", code, errOut, h.calls)
			}
			if got[0] != c.want {
				t.Errorf("context line = %q, want %q", got[0], c.want)
			}
		})
	}
}

func TestNobodyReadsAsAnObserverWithOneNotice(t *testing.T) {
	h := newHarness(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var got []string
	h.onRun = func(m tea.Model) { got = lines(drive(m, 100, 12)) }
	if code, _, errOut := runArgs("--fixture", fixtureDir(t)); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if want := "no Git identity: reading as an observer; pass --as <email>"; got[len(got)-2] != want {
		t.Errorf("status = %q, want %q", got[len(got)-2], want)
	}
}

// R reaches the role pane through the command's own wiring.
func TestRSwitchesTheRole(t *testing.T) {
	h := newHarness(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := fixtureDir(t)
	person := strings.Replace(tableau, `"window":1,`, `"person":"ada@example.org","window":1,`, 1)
	for name, body := range map[string]string{
		"viewers.json":        `{"ada@example.org":["owner","reviewer"]}`,
		"tableau-person.json": person,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var first, second []string
	h.onRun = func(m tea.Model) {
		m = drive(m, 100, 12)
		first = lines(m)
		second = lines(typed(m, tea.KeyPressMsg{Code: 'R', Text: "R"}))
	}
	if code, _, errOut := runArgs("--fixture", dir, "--as", "ada@example.org"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.HasSuffix(first[0], "viewer ada@example.org, owner") {
		t.Errorf("first context line = %q", first[0])
	}
	if !strings.HasSuffix(second[0], "viewer ada@example.org, reviewer") || second[len(second)-2] != "reading as reviewer, 2 of 2" {
		t.Errorf("after R: %q, status %q", second[0], second[len(second)-2])
	}
}
