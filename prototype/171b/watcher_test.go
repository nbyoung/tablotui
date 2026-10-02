package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const generous = 15 * time.Second

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.org",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.org")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo builds a repository on branch main with one commit and a .tableaux tree.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, ".tableaux/tasks/aaaa.yaml"), "title: A\n")
	write(t, filepath.Join(dir, ".tableaux/status/aaaa.yaml"), "gate: defined\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "first")
	return dir
}

func fastWatcher(dir string, tree bool) *Watcher {
	return &Watcher{Dir: dir, Ref: "HEAD", Tree: tree, Interval: 20 * time.Millisecond, Settle: 100 * time.Millisecond}
}

// expectChange runs act after Next starts waiting and returns the new fingerprint.
func expectChange(t *testing.T, w *Watcher, act func()) Fingerprint {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), generous)
	defer cancel()
	prev := w.Snapshot(ctx)
	done := make(chan Fingerprint, 1)
	go func() {
		fp, err := w.Next(ctx, prev)
		if err != nil {
			t.Errorf("Next: %v", err)
		}
		done <- fp
	}()
	time.Sleep(50 * time.Millisecond)
	act()
	fp := <-done
	if fp == prev {
		t.Fatal("Next returned the previous fingerprint")
	}
	return fp
}

func commit(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
}

func TestCommitOnBranch(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, false)
	var want string
	fp := expectChange(t, w, func() { commit(t, dir, "second"); want = runGit(t, dir, "rev-parse", "HEAD") })
	if fp.Commit != want || fp.Name != "main" {
		t.Fatalf("got %+v, want commit %s on main", fp, want)
	}
}

func TestCommitWithPackedRefs(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "pack-refs", "--all", "--prune")
	if _, err := os.Stat(filepath.Join(dir, ".git/refs/heads/main")); err == nil {
		t.Fatal("ref is still loose")
	}
	w := fastWatcher(dir, false)
	expectChange(t, w, func() { commit(t, dir, "after pack") })
	// Moving the ref by rewriting packed-refs itself, not a loose file.
	head := runGit(t, dir, "rev-parse", "HEAD")
	expectChange(t, w, func() {
		runGit(t, dir, "reset", "-q", "--hard", "HEAD~1")
		runGit(t, dir, "pack-refs", "--all", "--prune")
	})
	if got := runGit(t, dir, "rev-parse", "HEAD"); got == head {
		t.Fatal("reset did not move HEAD")
	}
}

func TestCheckoutOtherBranch(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "other")
	commit(t, dir, "on other")
	runGit(t, dir, "checkout", "-q", "main")
	w := fastWatcher(dir, false)
	fp := expectChange(t, w, func() { runGit(t, dir, "checkout", "-q", "other") })
	if fp.Name != "other" {
		t.Fatalf("name %q, want other", fp.Name)
	}
}

func TestCheckoutBranchAtSameCommit(t *testing.T) {
	dir := newRepo(t)
	runGit(t, dir, "branch", "twin")
	w := fastWatcher(dir, false)
	fp := expectChange(t, w, func() { runGit(t, dir, "checkout", "-q", "twin") })
	if fp.Name != "twin" {
		t.Fatalf("name %q, want twin", fp.Name)
	}
}

func TestDetachedHead(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "second")
	w := fastWatcher(dir, false)
	fp := expectChange(t, w, func() { runGit(t, dir, "checkout", "-q", "--detach", "HEAD~1") })
	if fp.Name != "HEAD" {
		t.Fatalf("name %q, want HEAD for a detached head", fp.Name)
	}
	expectChange(t, w, func() { commit(t, dir, "detached commit") })
}

func TestLinkedWorktree(t *testing.T) {
	main := newRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, main, "worktree", "add", "-q", "-b", "side", wt)
	if fi, err := os.Stat(filepath.Join(wt, ".git")); err != nil || fi.IsDir() {
		t.Fatal(".git is not a file in the linked worktree")
	}
	w := fastWatcher(wt, false)
	fp := expectChange(t, w, func() { commit(t, wt, "in worktree") })
	if fp.Name != "side" {
		t.Fatalf("name %q, want side", fp.Name)
	}
	// A commit in the main checkout moves main, not the worktree's HEAD.
	base := w.Snapshot(context.Background())
	commit(t, main, "in main")
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := w.Next(ctx, base); err == nil {
		t.Fatal("unexpected change")
	}
}

func TestSubmodule(t *testing.T) {
	sub := newRepo(t)
	super := newRepo(t)
	runGit(t, super, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "libs/sub")
	runGit(t, super, "commit", "-q", "-m", "add submodule")
	dir := filepath.Join(super, "libs/sub")
	if fi, err := os.Stat(filepath.Join(dir, ".git")); err != nil || fi.IsDir() {
		t.Fatal(".git is not a file in the submodule")
	}
	w := fastWatcher(dir, true)
	expectChange(t, w, func() { commit(t, dir, "in submodule") })
	// Open resolves the submodule's own root.
	_, l, _, err := Open(dir, "", false)
	if err != nil || l.Dir == super {
		t.Fatalf("Open: dir %q err %v", l.Dir, err)
	}
}

func TestUncommittedTableauxEdits(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, true)
	status := filepath.Join(dir, ".tableaux/status/aaaa.yaml")
	expectChange(t, w, func() { write(t, status, "gate: defined\nstate: at_risk\n") })
	expectChange(t, w, func() { write(t, filepath.Join(dir, ".tableaux/tasks/bbbb.yaml"), "title: B\n") })
	expectChange(t, w, func() { _ = os.Remove(filepath.Join(dir, ".tableaux/tasks/bbbb.yaml")) })
	// Same size, same second: only the mtime differs.
	expectChange(t, w, func() {
		write(t, status, "gate: defined\nstate: nominal\n")
		_ = os.Chtimes(status, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	})
	// With the tree not watched, the same edit is invisible.
	w2 := fastWatcher(dir, false)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := w2.Next(ctx, w2.Snapshot(context.Background())); err == nil {
		t.Fatal("HEAD-only watcher saw a working-tree edit")
	}
}

func TestBurstCollapses(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, true)
	w.Settle = 400 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), generous)
	defer cancel()
	prev := w.Snapshot(ctx)
	go func() {
		time.Sleep(50 * time.Millisecond)
		for _, n := range []string{"b", "c", "d", "e", "f"} {
			write(t, filepath.Join(dir, ".tableaux/tasks/"+n+".yaml"), "title: "+n+"\n")
			time.Sleep(20 * time.Millisecond)
		}
	}()
	fp, err := w.Next(ctx, prev)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(listFiles(t, dir)); got != 7 {
		t.Fatalf("Next returned after %d files, want all 7", got)
	}
	// The burst is spent: no second result.
	short, cancel2 := context.WithTimeout(ctx, time.Second)
	defer cancel2()
	if _, err := w.Next(short, fp); err == nil {
		t.Fatal("a second change followed the burst")
	}
}

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, ".tableaux/*/*.yaml"))
	return m
}

func TestBrokenRepositoryIsAChange(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, false)
	fp := expectChange(t, w, func() { _ = os.Rename(filepath.Join(dir, ".git"), filepath.Join(dir, ".git.off")) })
	if fp.Err == "" {
		t.Fatal("no error recorded")
	}
	expectChange(t, w, func() { _ = os.Rename(filepath.Join(dir, ".git.off"), filepath.Join(dir, ".git")) })
}

func TestCancel(t *testing.T) {
	dir := newRepo(t)
	w := fastWatcher(dir, false)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { _, err := w.Next(ctx, w.Snapshot(ctx)); errc <- err }()
	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("no error on cancel")
		}
	case <-time.After(generous):
		t.Fatal("Next ignored cancel")
	}
}

func TestLoaderAndOpenAt(t *testing.T) {
	dir := newRepo(t)
	first := runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "tag", "v1")
	write(t, filepath.Join(dir, ".tableaux/tasks/aaaa.yaml"), "title: changed\n")
	runGit(t, dir, "commit", "-q", "-a", "-m", "second")
	write(t, filepath.Join(dir, ".tableaux/tasks/zzzz.yaml"), "title: uncommitted\n")

	ctx := context.Background()
	src, l, _, err := Open(dir, "v1", false)
	if err != nil || src.Commit != first || src.Watch || src.Worktree {
		t.Fatalf("pinned: %+v %v", src, err)
	}
	pinned, err := l.Load(ctx, src)
	if err != nil || pinned.Commit != first || pinned.Files != 2 {
		t.Fatalf("pinned load: %+v %v", pinned, err)
	}
	live, _, _, _ := Open(dir, "", false)
	lv, err := l.Load(ctx, live)
	if err != nil || lv.Files != 3 || lv.Digest == pinned.Digest {
		t.Fatalf("live load: %+v %v", lv, err)
	}
	fol, _, w, _ := Open(dir, "main", true)
	if !fol.Watch || fol.Worktree || fol.Commit != "" || w.Tree {
		t.Fatalf("follow: %+v", fol)
	}
	fv, _ := l.Load(ctx, fol)
	if fv.Files != 2 || fv.Commit == first {
		t.Fatalf("follow load: %+v", fv)
	}
	if _, _, _, err := Open(dir, "nope", false); err == nil {
		t.Fatal("unknown ref accepted")
	}
}
