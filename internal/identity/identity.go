// Package identity names the person at the keyboard: the email Git would
// commit with, by tablo's rule, unless the command line names another.
package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nbyoung/tablo"
)

// ErrUsage marks an error that the command reports with exit status 2.
var ErrUsage = errors.New("usage")

// Email returns the viewer's email: as, when it is not empty; else what
// tablo.DefaultViewer gives for dir and the environment's GIT_AUTHOR_EMAIL,
// the one rule of tablo and the three front ends: that variable when it is
// set, else the user.email of Git's configuration in dir, else "", which is
// nobody, and a value that is no email is nobody too. It refuses an as that
// is no email, one "@" with text on both sides and no space, with an error
// that wraps ErrUsage.
func Email(ctx context.Context, dir, as string) (string, error) {
	if as != "" {
		name, host, ok := strings.Cut(as, "@")
		if !ok || name == "" || host == "" || strings.ContainsAny(as, " \t\n") || strings.Contains(host, "@") {
			return "", fmt.Errorf("%w: --as %s: not an email", ErrUsage, as)
		}
		return as, nil
	}
	return tablo.DefaultViewer(ctx, dir, os.Getenv("GIT_AUTHOR_EMAIL")), nil
}
