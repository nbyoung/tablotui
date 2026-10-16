package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nbyoung/tablotui/internal/view"
)

// File serves fixtures from Dir. The file name is the view, then "-window<n>"
// when Window is not 1, "-historical" when Historical, "-person" when Person
// is set, then ".json". A file holds tablo's envelope of the view. A missing
// file is an error. Tableau echoes the viewer and the role of a request into
// the envelope's viewer email and role of the tableau it decodes, and Name
// ignores both.
type File struct{ Dir string }

// Name returns the file name that serves r, without its directory.
func (f File) Name(r Request) string {
	name := r.View
	if name == "" {
		name = "tableau"
	}
	if r.Window != 1 {
		name += fmt.Sprintf("-window%d", r.Window)
	}
	if r.Historical {
		name += "-historical"
	}
	if r.Person != "" {
		name += "-person"
	}
	return name + ".json"
}

// Tableau implements Tableaux.
func (f File) Tableau(ctx context.Context, r Request) (view.Tableau, error) {
	if err := ctx.Err(); err != nil {
		return view.Tableau{}, err
	}
	file, err := os.Open(filepath.Join(f.Dir, f.Name(r)))
	if err != nil {
		return view.Tableau{}, err
	}
	defer func() { _ = file.Close() }()
	t, err := view.Decode(file)
	if err != nil {
		return view.Tableau{}, fmt.Errorf("%s: %w", f.Name(r), err)
	}
	if r.Viewer != "" {
		t.Viewer.Email = r.Viewer
	}
	if r.Role != "" {
		t.Params.Role = r.Role
	}
	return t, nil
}
