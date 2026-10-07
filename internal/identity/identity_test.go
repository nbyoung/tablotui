package identity

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// isolate keeps the host's Git identity out of a test: no global or system
// configuration, no author variable. It returns an empty directory.
func isolate(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	return t.TempDir()
}

// repo makes a repository in dir that states user.email.
func repo(t *testing.T, dir, email string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", email}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the path")
	}
}

func email(t *testing.T, dir, as string) string {
	t.Helper()
	got, err := Email(context.Background(), dir, as)
	if err != nil {
		t.Fatalf("Email(%q, %q): %v", dir, as, err)
	}
	return got
}

// T15: Email.
func TestNoIdentityIsNobody(t *testing.T) {
	needGit(t)
	dir := isolate(t)
	if got := email(t, dir, ""); got != "" {
		t.Errorf("Email = %q, want nobody", got)
	}
}

func TestEmailFromARepository(t *testing.T) {
	needGit(t)
	dir := isolate(t)
	repo(t, dir, "ada@example.org")
	if got := email(t, dir, ""); got != "ada@example.org" {
		t.Errorf("Email = %q", got)
	}
}

func TestTheAuthorVariableComesBeforeTheRepository(t *testing.T) {
	needGit(t)
	dir := isolate(t)
	repo(t, dir, "ada@example.org")
	t.Setenv("GIT_AUTHOR_EMAIL", "  ben@example.org\n")
	if got := email(t, dir, ""); got != "ben@example.org" {
		t.Errorf("Email = %q", got)
	}
}

func TestAsComesBeforeBoth(t *testing.T) {
	needGit(t)
	dir := isolate(t)
	repo(t, dir, "ada@example.org")
	t.Setenv("GIT_AUTHOR_EMAIL", "ben@example.org")
	if got := email(t, dir, "eve@example.org"); got != "eve@example.org" {
		t.Errorf("Email = %q", got)
	}
}

func TestMalformedAsIsAUsageError(t *testing.T) {
	dir := isolate(t)
	for _, as := range []string{"ada", "@example.org", "ada@", "ada lovelace@example.org", "ada@example@org"} {
		got, err := Email(context.Background(), dir, as)
		if !errors.Is(err, ErrUsage) || got != "" {
			t.Errorf("Email(%q) = %q, %v; want a usage error", as, got, err)
		}
	}
}

func TestNoGitIsNobody(t *testing.T) {
	dir := isolate(t)
	t.Setenv("PATH", "")
	if got := email(t, dir, ""); got != "" {
		t.Errorf("Email = %q, want nobody", got)
	}
}
