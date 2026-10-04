package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// tableauxDir is the plan's directory, relative to the repository root.
const tableauxDir = ".tableaux"

// Source says what the interface views.
type Source struct {
	Ref      string // the ref named: "HEAD" when live
	Commit   string // the commit a pinned view fixes; empty otherwise
	Worktree bool   // read .tableaux from the working tree, not from the commit
	Watch    bool   // reload when the source changes
}

// LiveSource views the working tree on HEAD and follows both.
func LiveSource() Source { return Source{Ref: "HEAD", Worktree: true, Watch: true} }

// Mode names the source for the status screen.
func (s Source) Mode() string {
	switch {
	case s.Worktree:
		return "live: watching HEAD and .tableaux"
	case s.Watch:
		return "following " + s.Ref + ": watching the ref"
	default:
		return "pinned to " + s.Ref + ": no refresh"
	}
}

// Fingerprint is a cheap, comparable summary of what the watcher polls. An
// error is part of the fingerprint, so a repository that breaks or heals is a
// change like any other.
type Fingerprint struct {
	Commit string
	Name   string // the branch HEAD names, or "HEAD" when detached
	Tree   string // stat digest of .tableaux; empty when not watched
	Err    string
}

// Watcher polls the repository with the standard library and git.
type Watcher struct {
	Dir      string        // the repository root
	Ref      string        // "HEAD" or a ref to follow
	Tree     bool          // also fingerprint .tableaux in the working tree
	Interval time.Duration // between polls
	Settle   time.Duration // a change must hold this long before it is reported
}

// NewWatcher returns a watcher for a source with the default timings.
func NewWatcher(dir string, s Source) *Watcher {
	return &Watcher{Dir: dir, Ref: s.Ref, Tree: s.Worktree,
		Interval: time.Second, Settle: 250 * time.Millisecond}
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(strings.SplitN(msg, "\n", 2)[0])
	}
	return strings.TrimSpace(out.String()), nil
}

// resolve returns the commit a ref names and the name to show for it. One git
// process answers both for HEAD, through the same lookup git itself uses, so
// loose refs, packed-refs, detached HEADs, linked worktrees and submodules
// need no case of their own.
func resolve(ctx context.Context, dir, ref string) (commit, name string, err error) {
	if ref == "HEAD" {
		out, err := git(ctx, dir, "rev-parse", "HEAD", "--abbrev-ref", "HEAD")
		if err != nil {
			return "", "", err
		}
		f := strings.Fields(out)
		if len(f) != 2 {
			return "", "", fmt.Errorf("unexpected rev-parse output %q", out)
		}
		return f[0], f[1], nil
	}
	out, err := git(ctx, dir, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return out, ref, err
}

// Snapshot reads the fingerprint once.
func (w *Watcher) Snapshot(ctx context.Context) Fingerprint {
	var fp Fingerprint
	var err error
	fp.Commit, fp.Name, err = resolve(ctx, w.Dir, w.Ref)
	if err != nil {
		return Fingerprint{Err: err.Error()}
	}
	if w.Tree {
		fp.Tree = statDigest(filepath.Join(w.Dir, tableauxDir))
	}
	return fp
}

// statDigest hashes the names, sizes and modification times under dir. It
// reads no file, so its cost is one stat per entry. It misses an edit that
// keeps the size and lands within the file system's timestamp granularity.
func statDigest(dir string) string {
	h := sha256.New()
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\x00%v\n", rel, info.Size(), info.ModTime().UnixNano(), info.Mode().Type())
		n++
		return nil
	})
	if n == 0 {
		return "none"
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Next blocks until the fingerprint differs from prev and has held steady for
// Settle, then returns it. A burst of changes, such as a commit that rewrites
// several files, so yields one result. A change that reverts inside the
// settle window yields none. Next returns the context's error when cancelled.
func (w *Watcher) Next(ctx context.Context, prev Fingerprint) (Fingerprint, error) {
	for {
		cur := w.Snapshot(ctx)
		if err := ctx.Err(); err != nil {
			return prev, err
		}
		if cur != prev {
			for {
				if err := sleep(ctx, w.Settle); err != nil {
					return prev, err
				}
				next := w.Snapshot(ctx)
				if next == cur {
					break
				}
				cur = next
			}
			if cur != prev {
				return cur, nil
			}
		}
		if err := sleep(ctx, w.Interval); err != nil {
			return prev, err
		}
	}
}

// View is what a load returns: the stand-in for tablo's view data.
type View struct {
	Commit string
	Ref    string
	Digest string // of the .tableaux tree as loaded
	Files  int
}

// Loader is the seam where tablo's library call goes later.
type Loader interface {
	Load(ctx context.Context, src Source) (View, error)
}

// GitLoader is the stand-in: it resolves the commit and digests .tableaux.
type GitLoader struct{ Dir string }

// Load resolves the commit and digests the tree the source selects.
func (l GitLoader) Load(ctx context.Context, src Source) (View, error) {
	commit := src.Commit
	if commit == "" {
		var err error
		if commit, _, err = resolve(ctx, l.Dir, src.Ref); err != nil {
			return View{}, err
		}
	}
	v := View{Commit: commit, Ref: src.Ref}
	h := sha256.New()
	if src.Worktree {
		root := filepath.Join(l.Dir, tableauxDir)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			_, _ = fmt.Fprintf(h, "%s\x00", rel)
			_, _ = h.Write(b)
			v.Files++
			return nil
		})
		if err != nil {
			return View{}, fmt.Errorf("read %s: %w", tableauxDir, err)
		}
	} else {
		out, err := git(ctx, l.Dir, "ls-tree", "-r", commit, "--", tableauxDir)
		if err != nil {
			return View{}, err
		}
		if out != "" {
			v.Files = strings.Count(out, "\n") + 1
			_, _ = h.Write([]byte(out))
		}
	}
	if v.Files == 0 {
		return View{}, fmt.Errorf("no %s tree at %s", tableauxDir, src.Ref)
	}
	v.Digest = hex.EncodeToString(h.Sum(nil))[:12]
	return v, nil
}

// Open builds the source, the loader and the watcher for the flags. at is the
// ref of --at; follow keeps the view on that ref as it moves.
func Open(dir, at string, follow bool) (Source, *GitLoader, *Watcher, error) {
	ctx := context.Background()
	top, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Source{}, nil, nil, err
	}
	src := LiveSource()
	if at != "" {
		commit, _, err := resolve(ctx, top, at)
		if err != nil {
			return Source{}, nil, nil, fmt.Errorf("ref %q: %w", at, err)
		}
		src = Source{Ref: at, Commit: commit, Watch: follow}
		if follow {
			src.Commit = ""
		}
	}
	return src, &GitLoader{Dir: top}, NewWatcher(top, src), nil
}
