package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nbyoung/tablotui/internal/view"
)

// Viewers names the roles an email holds in the project at a ref. A call
// blocks; the role pane runs it in a command.
type Viewers interface {
	Viewer(ctx context.Context, ref, email string) (view.Viewer, error)
}

// viewersFile is the fixture that maps an email to its roles.
const viewersFile = "viewers.json"

// Viewer implements Viewers from viewers.json in Dir, an object that maps an
// email to its roles. A missing file, and an email the file lacks, give an
// observer.
func (f File) Viewer(ctx context.Context, _, email string) (view.Viewer, error) {
	if err := ctx.Err(); err != nil {
		return view.Viewer{}, err
	}
	v := view.Viewer{Email: email, Roles: []string{"observer"}}
	b, err := os.ReadFile(filepath.Join(f.Dir, viewersFile))
	if errors.Is(err, fs.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return view.Viewer{}, err
	}
	var all map[string][]string
	if err := json.Unmarshal(b, &all); err != nil {
		return view.Viewer{}, fmt.Errorf("%s: %w", viewersFile, err)
	}
	if roles := all[email]; len(roles) > 0 {
		v.Roles = roles
	}
	return v, nil
}
