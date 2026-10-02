package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var testEnv = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	"GIT_AUTHOR_NAME=Claude Sonnet 5.5", "GIT_AUTHOR_EMAIL=noreply@anthropic.com",
	"GIT_COMMITTER_NAME=Claude Sonnet 5.5", "GIT_COMMITTER_EMAIL=noreply@anthropic.com",
}

var testAgent = Agent{Model: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5"}

const gatesYAML = `gates:
  - { key: undefined, symbol: ❔, name: Undefined }
  - { key: defined,   symbol: 📝, name: Defined }
  - { key: function,  symbol: 🔧, name: Functional prototype }
  - { key: design,    symbol: 📐, name: Design }
states:
  - { key: undefined, symbol: ⚪ }
  - { key: nominal,   symbol: 🟢 }
  - { key: at_risk,   symbol: 🟡 }
  - { key: complete,  symbol: ✅ }
reasons:
  - { key: blocked, symbol: ⛔ }
  - { key: review,  symbol: 👓 }
`

func gitIn(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds a throwaway repository with three tasks, the first
// (07e0, the root) with no status, b618 at defined and c48a at function.
func fixture(t *testing.T) string {
	dir := t.TempDir()
	gitIn(t, dir, testEnv, "init", "-q", "-b", "main")
	put(t, dir, ".tableaux/gates.yaml", gatesYAML)
	put(t, dir, ".tableaux/tasks/07e0.yaml", "title: Root\ndescription: r\nassignee: a@example.org\n")
	put(t, dir, ".tableaux/tasks/b618.yaml", "title: Leaf one\ndescription: l\nassignee: a@example.org\nparent: { id: \"07e0\", order: 1 }\n")
	put(t, dir, ".tableaux/tasks/c48a.yaml", "title: Leaf two\ndescription: l\nassignee: a@example.org\nparent: { id: \"07e0\", order: 2 }\n")
	put(t, dir, ".tableaux/status/b618.yaml", "# kept\ngate: defined\nstate: nominal\n")
	put(t, dir, ".tableaux/status/c48a.yaml", "gate: function\nstate: at_risk\nreason: review\n")
	gitIn(t, dir, testEnv, "add", "-A")
	gitIn(t, dir, testEnv, "commit", "-q", "-m", "Start")
	return dir
}

func newTest(t *testing.T, dir string, env []string) model {
	m, err := newModel(dir, env, testAgent)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// step sends one message to Update and returns the model and the command.
func step(m model, msg tea.Msg) (model, tea.Cmd) {
	n, c := m.Update(msg)
	return n.(model), c
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press sends keys; when one returns a command, press runs it, as the
// runtime does, and sends the message back.
func press(t *testing.T, m model, keys ...string) model {
	t.Helper()
	for _, k := range keys {
		var c tea.Cmd
		m, c = step(m, key(k))
		if c != nil {
			m, _ = step(m, c())
		}
	}
	return m
}

func logLines(t *testing.T, dir string) string {
	return gitIn(t, dir, testEnv, "log", "--format=%H %s")
}

func clean(t *testing.T, dir string) {
	t.Helper()
	if s := gitIn(t, dir, testEnv, "status", "--porcelain"); s != "" {
		t.Errorf("git status not clean:\n%s", s)
	}
}

func parsedTrailers(t *testing.T, dir string) string {
	msg := gitIn(t, dir, testEnv, "log", "-1", "--format=%B")
	cmd := exec.Command("git", "interpret-trailers", "--parse")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// selectTask moves the cursor to the task with the id.
func selectTask(t *testing.T, m model, id string) model {
	t.Helper()
	for i, tk := range m.tasks {
		if tk.ID == id {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no task %s", id)
	return m
}

func TestRecordFlowConfirms(t *testing.T) {
	dir := fixture(t)
	m := selectTask(t, newTest(t, dir, testEnv), "b618")
	before := logLines(t, dir)

	m = press(t, m, "r")
	if m.mode != modeForm || m.form.gate != indexIn(m.gates, "defined") {
		t.Fatalf("form not open at the task's gate: mode %v", m.mode)
	}
	// gate: defined -> function; state stays nominal; skip reason; type a note.
	m = press(t, m, "right", "tab", "tab", "tab")
	for _, r := range "Shown and applied" {
		m, _ = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = press(t, m, "tab", "right") // self-review: yes

	// Enter composes in a command: Update does no git work.
	m, cmd := step(m, key("enter"))
	if m.mode != modeComposed || cmd == nil {
		t.Fatalf("Enter did not start composing: mode %v", m.mode)
	}
	m, _ = step(m, cmd())
	if m.mode != modePreview {
		t.Fatalf("no preview, mode %v err %v", m.mode, m.err)
	}
	v := m.View()
	for _, want := range []string{
		"Record task b618 at the function gate",
		"Reviewed: b618 function", "Model: claude-sonnet-5-5",
		"Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>",
		"-gate: defined", "+gate: function", "+note: Shown and applied", "y/Enter apply",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("preview lacks %q:\n%s", want, v)
		}
	}
	if logLines(t, dir) != before {
		t.Fatal("the log changed before confirmation")
	}
	clean(t, dir)

	// y returns a command and changes nothing itself.
	m, cmd = step(m, key("y"))
	if m.mode != modeApplying || cmd == nil {
		t.Fatalf("y did not start applying: mode %v", m.mode)
	}
	if logLines(t, dir) != before {
		t.Fatal("Update ran git")
	}
	m, _ = step(m, cmd())
	if m.mode != modeList || !strings.HasPrefix(m.status, "committed: ") {
		t.Fatalf("not back at the list: mode %v status %q err %v", m.mode, m.status, m.err)
	}
	if got := gitIn(t, dir, testEnv, "log", "-1", "--format=%s"); strings.TrimSpace(got) != "Record task b618 at the function gate" {
		t.Errorf("subject %q", got)
	}
	want := "Reviewed: b618 function\nModel: claude-sonnet-5-5\nCo-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>\n"
	if got := parsedTrailers(t, dir); got != want {
		t.Errorf("trailers %q want %q", got, want)
	}
	clean(t, dir)
	b, _ := os.ReadFile(filepath.Join(dir, ".tableaux/status/b618.yaml"))
	if string(b) != "# kept\ngate: function\nstate: nominal\nnote: Shown and applied\n" {
		t.Errorf("status file %q", b)
	}
	if tk := m.tasks[m.cursor]; tk.Gate != "function" {
		t.Errorf("list not reloaded: %+v", tk)
	}
}

func TestEnterConfirms(t *testing.T) {
	dir := fixture(t)
	m := selectTask(t, newTest(t, dir, testEnv), "c48a")
	m = press(t, m, "a") // reaffirm composes at once
	if m.mode != modePreview || !strings.Contains(m.View(), "no file changes") {
		t.Fatalf("reaffirm preview: mode %v\n%s", m.mode, m.View())
	}
	m = press(t, m, "enter")
	if want := "Reaffirmed: c48a\nModel: claude-sonnet-5-5\nCo-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>\n"; parsedTrailers(t, dir) != want {
		t.Errorf("trailers %q", parsedTrailers(t, dir))
	}
	clean(t, dir)
}

func TestReviewAndAuthorise(t *testing.T) {
	dir := fixture(t)
	m := selectTask(t, newTest(t, dir, testEnv), "b618")
	m = press(t, m, "v", "right", "enter", "y") // defined -> function
	if got := parsedTrailers(t, dir); !strings.HasPrefix(got, "Reviewed: b618 function\n") {
		t.Errorf("review trailers %q", got)
	}
	_ = press(t, m, "u", "y")
	if got := parsedTrailers(t, dir); !strings.HasPrefix(got, "Authorised: b618\n") {
		t.Errorf("authorise trailers %q", got)
	}
	clean(t, dir)
	if n := strings.Count(logLines(t, dir), "\n"); n != 3 {
		t.Errorf("want 3 commits, got %d", n)
	}
}

func TestCancelChangesNothing(t *testing.T) {
	for _, cancel := range []string{"n", "esc"} {
		dir := fixture(t)
		before := logLines(t, dir)
		m := selectTask(t, newTest(t, dir, testEnv), "b618")
		m = press(t, m, "r", "right", "enter")
		if m.mode != modePreview {
			t.Fatalf("mode %v", m.mode)
		}
		m = press(t, m, cancel)
		if m.mode != modeList || !strings.Contains(m.status, "cancelled") {
			t.Errorf("%s: mode %v status %q", cancel, m.mode, m.status)
		}
		if logLines(t, dir) != before {
			t.Errorf("%s: the log changed", cancel)
		}
		clean(t, dir)
		// Esc in the form cancels too.
		m = press(t, m, "r", "esc")
		if m.mode != modeList {
			t.Errorf("form esc: mode %v", m.mode)
		}
		clean(t, dir)
	}
}

func TestInputErrorsShown(t *testing.T) {
	dir := fixture(t)
	m := selectTask(t, newTest(t, dir, testEnv), "b618")
	m.gates = append(m.gates, "bogus") // a gate gates.yaml does not hold
	m = press(t, m, "v", "right", "right", "right", "enter")
	if m.mode != modeError || !strings.Contains(m.View(), `gate "bogus" is not in gates.yaml`) {
		t.Errorf("mode %v\n%s", m.mode, m.View())
	}
	clean(t, dir)
}

// failure runs a confirmed action that git refuses and checks that the model
// shows the error and the repository is as it was.
func failure(t *testing.T, dir string, env []string, keys []string, wantErr string) {
	t.Helper()
	before := logLines(t, dir)
	files := map[string]string{}
	for _, p := range []string{"b618", "c48a"} {
		b, _ := os.ReadFile(filepath.Join(dir, statusPath(p)))
		files[p] = string(b)
	}
	m := selectTask(t, newTest(t, dir, env), "b618")
	m = press(t, m, keys...)
	if m.mode != modeError {
		t.Fatalf("mode %v, want error; status %q", m.mode, m.status)
	}
	if v := m.View(); !strings.Contains(v, wantErr) || !strings.Contains(v, "The repository is as it was") {
		t.Errorf("view lacks %q:\n%s", wantErr, v)
	}
	if logLines(t, dir) != before {
		t.Error("the log changed")
	}
	clean(t, dir)
	for p, want := range files {
		b, _ := os.ReadFile(filepath.Join(dir, statusPath(p)))
		if string(b) != want {
			t.Errorf("%s changed: %q", p, b)
		}
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Errorf("Enter did not dismiss the error: %v", m.mode)
	}
}

func TestFailureNoIdentity(t *testing.T) {
	dir := fixture(t)
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=user.useConfigOnly", "GIT_CONFIG_VALUE_0=true",
		"GIT_AUTHOR_NAME=", "GIT_AUTHOR_EMAIL=", "GIT_COMMITTER_NAME=", "GIT_COMMITTER_EMAIL=", "EMAIL="}
	failure(t, dir, env, []string{"r", "right", "enter", "y"}, "empty ident name")
}

func TestFailureHookRejects(t *testing.T) {
	dir := fixture(t)
	put(t, dir, ".git/hooks/commit-msg", "#!/bin/sh\necho 'rejected by hook' >&2\nexit 1\n")
	if err := os.Chmod(filepath.Join(dir, ".git/hooks/commit-msg"), 0o755); err != nil {
		t.Fatal(err)
	}
	failure(t, dir, testEnv, []string{"r", "right", "enter", "y"}, "rejected by hook")
}

func TestFailureNothingToCommit(t *testing.T) {
	dir := fixture(t)
	// Record the status as it already stands: the file does not change.
	failure(t, dir, testEnv, []string{"r", "enter", "y"}, "nothing to commit")
}

func TestFailureNewFileRemoved(t *testing.T) {
	// A status file that did not exist is removed again after a failed commit.
	dir := fixture(t)
	put(t, dir, ".git/hooks/commit-msg", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(dir, ".git/hooks/commit-msg"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := selectTask(t, newTest(t, dir, testEnv), "07e0") // no status file
	m = press(t, m, "r", "right", "enter", "y")
	if m.mode != modeError {
		t.Fatalf("mode %v", m.mode)
	}
	if _, err := os.Stat(filepath.Join(dir, statusPath("07e0"))); err == nil {
		t.Error("the new status file stays")
	}
	clean(t, dir)
}

func TestSample(t *testing.T) {
	dir := fixture(t)
	m := selectTask(t, newTest(t, dir, testEnv), "b618")
	m = press(t, m, "r")
	t.Logf("form:\n%s", m.View())
	m = press(t, m, "right", "tab", "tab", "tab", "enter")
	t.Logf("preview:\n%s", m.View())
	m = press(t, m, "n")
	t.Logf("cancelled:\n%s", m.View())
}
