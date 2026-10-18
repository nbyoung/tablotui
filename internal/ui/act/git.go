package act

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// GitUncommitted lists the paths under dir/.tableaux, relative to dir and
// sorted, that differ from HEAD or that Git does not track. It runs git twice
// and writes nothing.
func GitUncommitted(ctx context.Context, dir string) ([]string, error) {
	changed, err := gitPaths(ctx, dir, "diff", "--name-only", "-z", "--relative", "HEAD", "--", ".tableaux")
	if err != nil {
		return nil, err
	}
	untracked, err := gitPaths(ctx, dir, "ls-files", "-z", "--others", "--exclude-standard", "--", ".tableaux")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range append(changed, untracked...) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// gitPaths runs git in dir with args, which end in a NUL-separated list of
// paths, and returns the paths. Git takes no optional lock, asks for nothing
// on the terminal and speaks C. A run that fails is the error
// "git <subcommand>: <git's first line>".
func gitPaths(ctx context.Context, dir string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		text := firstLine(stderr.String())
		if text == "" {
			text = firstLine(err.Error())
		}
		return nil, fmt.Errorf("git %s: %s", args[0], text)
	}
	var paths []string
	for _, p := range strings.Split(out.String(), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// firstLine returns the first line of s that is not blank, trimmed.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
