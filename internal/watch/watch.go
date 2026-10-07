// Package watch reports when the inputs of a Tableaux view change. It uses the
// standard library and the git executable, and it reads no file under .tableaux.
package watch

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

// Timings of a watcher made by New.
const (
	// DefaultInterval separates two polls while nothing changes.
	DefaultInterval = time.Second
	// DefaultSettle is how long a changed state must read the same, twice,
	// before it counts.
	DefaultSettle = 250 * time.Millisecond
	// DefaultMaxSettle bounds the settling: after this long without two equal
	// reads, the watcher reports the latest.
	DefaultMaxSettle = 2 * time.Second
)

// tableauxDir is the plan's directory, relative to the project directory.
const tableauxDir = ".tableaux"

// absent is the Tree of a live state whose project has no .tableaux directory.
const absent = "absent"

// ErrUsage marks an error of Open that the command reports with exit status 2.
var ErrUsage = errors.New("usage")

// Source says what the view reads.
type Source struct {
	Dir string // the project directory, absolute: the one that holds .tableaux
	Ref string // the revision --ref names; empty for the working tree on HEAD
}

// Live reports whether the view reads the working tree.
func (s Source) Live() bool { return s.Ref == "" }

// Open checks the flags and returns the source. It makes dir absolute. When at
// starts with a hyphen or holds "..", it wraps ErrUsage as "usage: --ref <ref>:
// name one commit". It fails when dir is not inside a Git repository ("<dir>:
// <git's message>") or when at names no commit ("--ref <ref>: no such commit").
//
// The directory Open returns is the one the caller names. A caller whose Loader
// resolves the project at or above it sets Source.Dir to that directory before
// it calls New, since the watcher watches the directory the Loader resolves.
func Open(ctx context.Context, dir, at string) (Source, error) {
	if strings.HasPrefix(at, "-") || strings.Contains(at, "..") {
		return Source{}, fmt.Errorf("%w: --ref %s: name one commit", ErrUsage, at)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Source{}, err
	}
	if _, err := runGit(ctx, abs, "rev-parse", "--git-dir"); err != nil {
		return Source{}, fmt.Errorf("%s: %s", abs, trimFatal(err.Error()))
	}
	if at != "" {
		if _, err := runGit(ctx, abs, "rev-parse", "--verify", "--end-of-options", at+"^{commit}"); err != nil {
			return Source{}, fmt.Errorf("--ref %s: no such commit", at)
		}
	}
	return Source{Dir: abs, Ref: at}, nil
}

// State is what a load reads, in a form that two polls compare with ==.
type State struct {
	Commit string // the commit in view, 40 or 64 hexadecimal digits
	Branch string // live: the branch HEAD names; empty when detached or with --ref
	Refs   string // the SHA-256, in hexadecimal, of the for-each-ref output
	Tree   string // live: the digest of .tableaux, or "absent"; empty with --ref
	Err    string // git's message when the commit does not resolve; the other fields are then empty

	// Modules is the SHA-256, in hexadecimal, of the HEAD of each submodule
	// checkout that SetSubmodules names; empty with --ref, or when none is named.
	Modules string
}

// runner runs git in dir and returns its standard output, trimmed. On failure
// the error holds the first line of git's standard error.
type runner func(ctx context.Context, dir string, args ...string) (string, error)

// runGit is the runner of a watcher made by New: it runs the git executable as
// "git -C dir args" under ctx, with the settings that keep git quiet and
// stable.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		first, _, _ := strings.Cut(msg, "\n")
		return "", errors.New(first)
	}
	return strings.TrimSpace(out.String()), nil
}

// trimFatal removes the "fatal: " that git puts before a message.
func trimFatal(msg string) string { return strings.TrimPrefix(msg, "fatal: ") }

// Snapshot reads the state once. It never fails: an error becomes State.Err.
//
// It runs two git processes, plus one per submodule that SetSubmodules names
// while the source is live, and one walk of .tableaux.
func (w *Watcher) Snapshot(ctx context.Context) State {
	var st State
	if w.Source.Live() {
		out, err := w.run(ctx, w.Source.Dir, "rev-parse", "HEAD", "--abbrev-ref", "HEAD")
		if err != nil {
			return State{Err: trimFatal(err.Error())}
		}
		f := strings.Fields(out)
		if len(f) != 2 {
			return State{Err: fmt.Sprintf("unexpected rev-parse output %q", out)}
		}
		st.Commit = f[0]
		if f[1] != "HEAD" {
			st.Branch = f[1]
		}
	} else {
		out, err := w.run(ctx, w.Source.Dir, "rev-parse", "--verify", "--end-of-options", w.Source.Ref+"^{commit}")
		if err != nil {
			return State{Err: trimFatal(err.Error())}
		}
		st.Commit = out
	}
	out, err := w.run(ctx, w.Source.Dir, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return State{Err: trimFatal(err.Error())}
	}
	sum := sha256.Sum256([]byte(out))
	st.Refs = hex.EncodeToString(sum[:])
	if w.Source.Live() {
		st.Tree = treeDigest(filepath.Join(w.Source.Dir, tableauxDir))
		st.Modules = w.modulesDigest(ctx)
	}
	return st
}

// modulesDigest hashes the HEAD of each named submodule checkout, one git
// process each. A checkout that git cannot read contributes its message, so
// that its failure and its healing are changes too.
func (w *Watcher) modulesDigest(ctx context.Context) string {
	mods := w.submodules()
	if len(mods) == 0 {
		return ""
	}
	h := sha256.New()
	for _, m := range mods {
		dir := m
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(w.Source.Dir, dir)
		}
		head, err := w.run(ctx, dir, "rev-parse", "HEAD")
		if err != nil {
			head = "error: " + trimFatal(err.Error())
		}
		_, _ = fmt.Fprintf(h, "%s\x00%s\n", filepath.ToSlash(m), head)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// treeDigest hashes one line per entry under dir: for a directory its
// slash-separated relative path and "d"; for any other entry its path, size,
// modification time in nanoseconds and mode in octal. It reads no file. It
// skips an entry that vanishes during the walk, follows no symbolic link and
// returns "absent" when dir does not exist.
func treeDigest(dir string) string {
	if _, err := os.Lstat(dir); err != nil {
		return absent
	}
	h := sha256.New()
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			_, _ = fmt.Fprintf(h, "%s\x00d\n", rel)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\x00%o\n", rel, info.Size(), info.ModTime().UnixNano(), uint32(info.Mode()))
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}
