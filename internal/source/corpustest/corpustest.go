// Package corpustest gives the integrate tests their repositories: a copy of
// an entry of the built conformance corpus, which TABLEAUX_CORPUS names, and
// a clone of the umbrella's plan, which TABLEAUX_DIR names. Both sources are
// shared and read-only, so no Git command runs in either but a clone that
// reads it, and a test that lacks one skips with the reason.
package corpustest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Entry copies the built entry name of the corpus, with its .git and its
// submodule checkouts, into a directory of the test and returns the copy.
// It skips without TABLEAUX_CORPUS, without git, or when the entry is absent.
func Entry(t testing.TB, name string) string {
	t.Helper()
	corpus := os.Getenv("TABLEAUX_CORPUS")
	if corpus == "" {
		t.Skip("TABLEAUX_CORPUS names no built corpus")
	}
	needGit(t)
	from := filepath.Join(corpus, name)
	if _, err := os.Stat(from); err != nil {
		t.Skipf("the corpus holds no %s: %v", name, err)
	}
	dir := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("cp", "-a", from, dir).CombinedOutput(); err != nil {
		t.Skipf("cannot copy the entry: %v: %s", err, out)
	}
	return dir
}

// Labels returns the commits that the labels file of a corpus entry names,
// by label: W1 to W13 for weather-station.
func Labels(t testing.TB, name string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(os.Getenv("TABLEAUX_CORPUS"), name+".labels.txt"))
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if label, commit, ok := strings.Cut(line, " "); ok {
			labels[label] = commit
		}
	}
	return labels
}

// The four subprojects the umbrella pins.
var subprojects = []string{"tablo", "tabloio", "tablotui", "tableaud"}

// Umbrella clones the umbrella's repository into a directory of the test,
// with a clone that reads and never writes it, and checks out commit on the
// branch main, so that the plan reads "main at" that commit as the mockups
// do. Each subproject is a clone of the umbrella's module store, checked out
// at the commit's pin, since a recursive junction reads its subproject there,
// with its trunk main at the pin too, so that the plan reads the same
// whatever the store's own main holds on the host.
// It skips without TABLEAUX_DIR, without git, or without the module stores.
func Umbrella(t testing.TB, commit string) string {
	t.Helper()
	src := os.Getenv("TABLEAUX_DIR")
	if src == "" {
		t.Skip("TABLEAUX_DIR names no umbrella checkout")
	}
	needGit(t)
	for _, m := range subprojects {
		if _, err := os.Stat(filepath.Join(src, ".git", "modules", "subprojects", m)); err != nil {
			t.Skipf("the umbrella holds no module store of %s, and its plan reads the subprojects: %v", m, err)
		}
	}
	dir := filepath.Join(t.TempDir(), "tableaux")
	Git(t, "", "clone", "--quiet", "--shared", "--no-checkout", src, dir)
	Git(t, dir, "checkout", "--quiet", "-B", "main", commit)
	for _, m := range subprojects {
		path := "subprojects/" + m
		fields := strings.Fields(Git(t, dir, "ls-tree", commit, "--", path))
		if len(fields) < 3 || fields[0] != "160000" {
			t.Fatalf("%s pins no %s: %v", commit, path, fields)
		}
		sub := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.RemoveAll(sub); err != nil {
			t.Fatal(err)
		}
		Git(t, "", "clone", "--quiet", "--shared", "--no-checkout", filepath.Join(src, ".git", "modules", "subprojects", m), sub)
		Git(t, sub, "checkout", "--quiet", "--detach", fields[2])
		Git(t, sub, "update-ref", "refs/remotes/origin/main", fields[2])
	}
	return dir
}

// Git runs git in dir, or in the test's working directory when dir is "",
// with no global or system configuration, and returns its output trimmed.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", append([]string{"-c", "init.defaultBranch=main", "-c", "advice.detachedHead=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		msg := ""
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, msg)
	}
	return strings.TrimSpace(string(out))
}

func needGit(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the path")
	}
}
