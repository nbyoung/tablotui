package act

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/watch/watchtest"
)

// fixed is the Source of T19: the facts of the one task of watchtest.Repo,
// whose function junction the hand that commits reviews itself, trailers as
// tablo composes them, and a validator that finds nothing.
type fixed struct{ tablo action.Tablo }

func (*fixed) Project(context.Context) (action.Project, error) {
	return action.Project{
		Gates:   []string{"undefined", "defined", "function", "design"},
		States:  []string{"undefined", "nominal", "at_risk", "complete"},
		Reasons: []string{"blocked", "review"},
		Trunk:   "main",
	}, nil
}

func (*fixed) Task(_ context.Context, id string) (action.Task, error) {
	const me = "test@example.org"
	return action.Task{ID: id, Title: "A", Assignee: me, Authorities: []string{me},
		Junctions: []action.Junction{
			{Gate: "defined", Kind: "plain", Contributor: me},
			{Gate: "function", Kind: "plain", Contributor: me, Reviewer: me},
			{Gate: "design", Kind: "plain", Contributor: me},
		},
		Status: &action.Status{Gate: "defined"},
	}, nil
}

func (f *fixed) Trailer(ctx context.Context, kind, id, gate string) (string, error) {
	return f.tablo.Trailer(ctx, kind, id, gate)
}

func (*fixed) Validate(context.Context, string) ([]action.Diagnostic, error) { return nil, nil }

// T19: the actions over the real package, in a repository of the test.
func TestTheRealPackage(t *testing.T) {
	needGit(t)
	dir := watchtest.Repo(t)
	src := &fixed{}
	r := newRig(t, 100, 30, func(o *Options) {
		o.Repo = &action.Repo{Dir: dir, Source: src}
		o.Facts = src
		o.Dir = dir
		o.Uncommitted = nil
	})
	git := func(args ...string) string { t.Helper(); return watchtest.Git(t, dir, args...) }

	// r: defined to function, which the hand reviews itself.
	r.selects("aaaa", "").keys("r right enter")
	screen := r.screen()
	if !strings.Contains(screen, "    Reviewed: aaaa function") {
		t.Fatalf("the plan:\n%s", screen)
	}
	r.keys("y")
	if !strings.HasPrefix(r.status(), "committed ") || r.modeOpen() {
		t.Fatalf("y: %q\n%s", r.status(), r.screen())
	}
	if got := git("log", "-1", "--format=%B"); !strings.HasPrefix(got, "Record A (aaaa) at function") {
		t.Errorf("the commit's message %q", got)
	}
	trailers := git("log", "-1", "--format=%(trailers:only,unfold)")
	if got := strings.TrimSpace(trailers); got != "Reviewed: aaaa function" {
		t.Errorf("the commit's trailers %q", got)
	}
	parsed := strings.TrimSpace(gitInput(t, dir, git("log", "-1", "--format=%B"), "interpret-trailers", "--parse"))
	if parsed != "Reviewed: aaaa function" {
		t.Errorf("interpret-trailers --parse gives %q", parsed)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".tableaux/status/aaaa.yaml")); string(b) != "gate: function\nstate: nominal\n" {
		t.Errorf("the status file holds %q", b)
	}

	// a: the empty Reaffirmed: commit.
	head := git("rev-parse", "HEAD")
	r.keys("a y")
	if !strings.HasPrefix(r.status(), "committed ") {
		t.Fatalf("a y: %q\n%s", r.status(), r.screen())
	}
	if got := git("log", "-1", "--format=%s%n%(trailers:only)"); got != "Reaffirm A (aaaa)\nReaffirmed: aaaa" {
		t.Errorf("the reaffirmation %q", got)
	}
	if got := git("diff", "--name-only", head, "HEAD"); got != "" {
		t.Errorf("the reaffirmation changes %q", got)
	}

	// A failing pre-commit hook: not committed, and the checkout stands.
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	watchtest.Write(t, hook, "#!/bin/sh\necho 'pre-commit: tablo validate reports 1 error' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	head = git("rev-parse", "HEAD")
	r.keys("r right right enter y")
	if l := r.lines(); l[1] != "record aaaa: not committed" || !strings.Contains(r.screen(), "pre-commit: tablo validate reports 1 error") ||
		r.status() != "error: not committed: git commit: exit 1" {
		t.Errorf("the hook's failure:\n%s", r.screen())
	}
	if got := git("status", "--porcelain"); got != "" {
		t.Errorf("git status --porcelain: %q", got)
	}
	if got := git("rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved to %s", got)
	}
}

// gitInput runs git in dir with input on its standard input.
func gitInput(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
