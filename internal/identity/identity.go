// Package identity names the person at the keyboard: the email Git would
// commit with, unless the command line names another.
package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrUsage marks an error that the command reports with exit status 2.
var ErrUsage = errors.New("usage")

// Email returns the viewer's email: as, when it is not empty; else the
// environment's GIT_AUTHOR_EMAIL; else what "git -C dir config --get
// user.email" prints; else "", which is nobody. It refuses an as that is no
// email, one "@" with text on both sides and no space, with an error that
// wraps ErrUsage. A git that is missing, fails or states no email gives "".
func Email(ctx context.Context, dir, as string) (string, error) {
	if as != "" {
		name, host, ok := strings.Cut(as, "@")
		if !ok || name == "" || host == "" || strings.ContainsAny(as, " \t\n") || strings.Contains(host, "@") {
			return "", fmt.Errorf("%w: --as %s: not an email", ErrUsage, as)
		}
		return as, nil
	}
	if e := strings.TrimSpace(os.Getenv("GIT_AUTHOR_EMAIL")); e != "" {
		return e, nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "config", "--get", "user.email")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}
