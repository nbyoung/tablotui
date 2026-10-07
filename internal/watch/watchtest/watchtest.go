// Package watchtest holds the test clock of internal/watch and the helpers
// that build the Git repositories its tests read. It imports no package of
// this module, so a test inside internal/watch may use it.
package watchtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Clock is the test clock of internal/watch. It satisfies watch.Clock. Each
// call of After announces its duration on Asked and returns Tick, so a test
// knows which wait the watcher stands in before it acts, and a send on Tick
// releases the one waiter.
type Clock struct {
	Asked chan time.Duration // receives the duration of each After call; capacity 64
	Tick  chan time.Time     // After returns it; a send releases the one waiter
}

// NewClock returns a clock with its channels made.
func NewClock() *Clock {
	return &Clock{
		Asked: make(chan time.Duration, 64),
		Tick:  make(chan time.Time),
	}
}

// Now returns T.

// After announces d on Asked and returns Tick.
func (c *Clock) After(d time.Duration) <-chan time.Time {
	c.Asked <- d
	return c.Tick
}

// Env sets, for the test and every process it starts, the environment that
// makes Git independent of the host: no global or system configuration, and a
// fixed identity.
func Env(t testing.TB) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.org")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.org")
}

// Git runs git in dir and returns its trimmed standard output. It fails the
// test when git fails.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Write writes content to path, and makes the directories above it.
func Write(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Repo makes a repository on branch main in a temporary directory, with one
// commit that holds a .tableaux tree of two files, and returns its directory.
// It calls Env. initArgs go to "git init" after its fixed flags.
func Repo(t testing.TB, initArgs ...string) string {
	t.Helper()
	Env(t)
	dir := t.TempDir()
	Git(t, dir, append([]string{"init", "-q", "-b", "main"}, initArgs...)...)
	Write(t, filepath.Join(dir, ".tableaux/tasks/aaaa.yaml"), "title: A\n")
	Write(t, filepath.Join(dir, ".tableaux/status/aaaa.yaml"), "gate: defined\n")
	Git(t, dir, "add", ".")
	Git(t, dir, "commit", "-q", "-m", "first")
	return dir
}
