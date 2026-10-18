package act

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/watch/watchtest"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the path")
	}
}

// T15: GitUncommitted lists the paths under .tableaux that differ from HEAD
// or that Git does not track, relative to the directory, and writes nothing.
func TestGitUncommitted(t *testing.T) {
	needGit(t)
	dir := watchtest.Repo(t)
	ctx := t.Context()
	list := func(dir string) []string {
		t.Helper()
		got, err := GitUncommitted(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := list(dir); len(got) != 0 {
		t.Errorf("a clean checkout lists %q", got)
	}

	index, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	head := watchtest.Git(t, dir, "rev-parse", "HEAD")

	// An edit, an untracked file, and a file outside .tableaux.
	watchtest.Write(t, filepath.Join(dir, ".tableaux/status/aaaa.yaml"), "gate: design\n")
	watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/bbbb.yaml"), "title: B\n")
	watchtest.Write(t, filepath.Join(dir, "README.md"), "outside\n")
	want := []string{".tableaux/status/aaaa.yaml", ".tableaux/tasks/bbbb.yaml"}
	if got := list(dir); !slices.Equal(got, want) {
		t.Errorf("an edit and an untracked file: %q, want %q", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".git", "index")); string(b) != string(index) {
		t.Error("GitUncommitted wrote the index")
	}
	if got := watchtest.Git(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved to %s", got)
	}

	// A staged edit and a staged deletion.
	watchtest.Git(t, dir, "checkout", "-q", "--", ".tableaux/status/aaaa.yaml")
	if err := os.Remove(filepath.Join(dir, ".tableaux/tasks/bbbb.yaml")); err != nil {
		t.Fatal(err)
	}
	watchtest.Write(t, filepath.Join(dir, ".tableaux/status/aaaa.yaml"), "gate: function\n")
	watchtest.Git(t, dir, "add", ".tableaux/status/aaaa.yaml")
	watchtest.Git(t, dir, "rm", "-q", "--cached", ".tableaux/tasks/aaaa.yaml")
	want = []string{".tableaux/status/aaaa.yaml", ".tableaux/tasks/aaaa.yaml"}
	if got := list(dir); !slices.Equal(got, want) {
		t.Errorf("a staged edit and a staged deletion: %q, want %q", got, want)
	}

	// A project below the root lists its own paths alone, relative to it.
	watchtest.Git(t, dir, "reset", "-q", "--hard")
	sub := filepath.Join(dir, "sub")
	watchtest.Write(t, filepath.Join(sub, ".tableaux/tasks/cccc.yaml"), "title: C\n")
	watchtest.Git(t, dir, "add", ".")
	watchtest.Git(t, dir, "commit", "-q", "-m", "sub")
	watchtest.Write(t, filepath.Join(sub, ".tableaux/tasks/cccc.yaml"), "title: C2\n")
	watchtest.Write(t, filepath.Join(dir, ".tableaux/tasks/aaaa.yaml"), "title: A2\n")
	if got, want := list(sub), []string{".tableaux/tasks/cccc.yaml"}; !slices.Equal(got, want) {
		t.Errorf("a project below the root: %q, want %q", got, want)
	}

	// No repository is an error that names the subcommand.
	none := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(none))
	if _, err := GitUncommitted(ctx, none); err == nil || !strings.HasPrefix(err.Error(), "git diff: ") || strings.Contains(err.Error(), "\n") {
		t.Errorf("no repository gives %v", err)
	}
}
