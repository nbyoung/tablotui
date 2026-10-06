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
	for _, want := range []string{"usage: tablotui", "--settings", "--fixture", "--version", "-C"} {
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
