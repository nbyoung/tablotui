# Design f394: the model in full

This draft holds the Go text of a trial that implements [the design](../f394.md) on `main` at `21084d3`, outside the repository. The trial builds, passes `gofmt`, `go vet` and `golangci-lint run`, passes every test on `main` with one count changed, and passes the tests T1 to T16 of the design. The implementation may copy it. Where it and the design differ, the design stands.

## New files

### `internal/view/viewer.go`

```go
package view

// Viewer is the person at the keyboard as tablo sees them: the email, and the
// roles that email holds anywhere in the project, in tablo's order. Roles is
// "observer" alone when the email holds none.
type Viewer struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}
```

### `internal/source/viewer.go`

```go
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
```

### `internal/identity/identity.go`

```go
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
```

### `internal/ui/role/role.go`

```go
// Package role places the person at the keyboard: it asks which roles the
// viewer holds, arrives the grid where the first of them works, and moves it
// on when the viewer switches the role. It resolves no role itself.
package role

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view"
)

// ID is the identifier of the role pane. The pane never opens.
const ID = "role"

// The roles, as tablo names them.
const (
	Owner       = "owner"
	Authority   = "authority"
	Assignee    = "assignee"
	Contributor = "contributor"
	Agent       = "agent"
	Reviewer    = "reviewer"
	Observer    = "observer"
)

// naming is the order in which a stop takes its name and the cycle its order.
var naming = []string{Owner, Assignee, Contributor, Agent, Authority, Reviewer, Observer}

// place is where a role arrives.
type place int

const (
	whole  place = iota // the global tableau at glance
	corner              // the contextual tableau, person form
	marked              // the global tableau with the viewer's cells marked
)

func placeOf(role string) place {
	switch role {
	case Assignee, Contributor, Agent, Authority:
		return corner
	case Reviewer:
		return marked
	}
	return whole
}

// Stops returns the roles that R cycles through: of the roles held and
// observer, in the naming order, the first that leads to each place.
func Stops(roles []string) []string {
	var out []string
	seen := map[place]bool{}
	for _, r := range naming {
		if r != Observer && !slices.Contains(roles, r) {
			continue
		}
		if p := placeOf(r); !seen[p] {
			seen[p] = true
			out = append(out, r)
		}
	}
	return out
}

// Arrival is where the viewer starts in a role. An unknown role, and "",
// arrive as an observer does.
func Arrival(role, viewer, ref string) grid.Start {
	s := grid.Start{Request: source.Request{
		View: "tableau", Ref: ref, Window: 1, Level: "detail", Viewer: viewer, Role: role,
	}}
	switch placeOf(role) {
	case corner:
		s.Request.View = "context"
		s.Request.Person = viewer
		s.Level = grid.Detail
		s.Mine = true
	case marked:
		s.Request.Person = viewer
		s.Mine = true
	}
	return s
}

// SwitchMsg asks the role pane for the next stop.
type SwitchMsg struct{}

// Command binds R to the switch of role.
func Command() ui.Command {
	return ui.Command{
		Binding: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "role")),
		Run:     func(ui.SelectionMsg) tea.Cmd { return func() tea.Msg { return SwitchMsg{} } },
	}
}

// Options configures the role pane. An empty Viewer is nobody: an observer.
type Options struct {
	Source source.Viewers
	Viewer string
	Ref    string
}

// Pane holds the viewer, the stops and the stop in force. It implements
// ui.Pane, draws nothing and never opens.
type Pane struct {
	src      source.Viewers
	viewer   string
	ref      string
	seq      uint64
	stops    []string // nil until tablo names the roles
	at       int
	arrived  bool   // the grid has a start
	selected string // the task the grid selects
}

// New builds the role pane.
func New(o Options) Pane {
	p := Pane{src: o.Source, viewer: o.Viewer, ref: o.Ref, seq: 1}
	if p.viewer == "" {
		p.stops = []string{Observer}
		p.arrived = true
	}
	return p
}

// ID implements ui.Pane.
func (p Pane) ID() string { return ID }

// Keys implements ui.Pane: the pane has none.
func (p Pane) Keys() []key.Binding { return nil }

// Status implements ui.Pane.
func (p Pane) Status(ui.Context) string { return "" }

// View implements ui.Pane: h blank lines of w cells.
func (p Pane) View(w, h int, _ ui.Context) string {
	lines := make([]string, max(h, 0))
	for i := range lines {
		lines[i] = strings.Repeat(" ", max(w, 0))
	}
	return strings.Join(lines, "\n")
}

// Role returns the role in force, or "" before tablo names the roles.
func (p Pane) Role() string {
	if len(p.stops) == 0 {
		return ""
	}
	return p.stops[p.at]
}

// Init implements ui.Pane: nobody arrives as an observer at once; anyone else
// asks for the roles.
func (p Pane) Init(c ui.Context) tea.Cmd {
	if p.viewer == "" {
		return tea.Batch(
			start(Arrival(Observer, "", p.ref)),
			notice("no Git identity: reading as an observer; pass --as <email>", false),
		)
	}
	return p.ask(c)
}

func (p Pane) ask(c ui.Context) tea.Cmd {
	src, ref, email := p.src, p.ref, p.viewer
	return c.Load(ID, p.seq, func(ctx context.Context) (any, error) {
		if src == nil {
			return nil, fmt.Errorf("no source")
		}
		return src.Viewer(ctx, ref, email)
	})
}

func start(s grid.Start) tea.Cmd {
	return func() tea.Msg { return grid.StartMsg{Start: s} }
}

func notice(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ui.NoticeMsg{Text: text, Err: isErr} }
}

// Update implements ui.Pane.
func (p Pane) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SelectionMsg:
		p.selected = msg.Task
	case ui.ReloadMsg:
		if p.viewer != "" {
			p.seq++
			return p, p.ask(c)
		}
	case SwitchMsg:
		return p.next(c)
	case ui.LoadedMsg:
		if msg.Pane == ID && msg.Seq == p.seq {
			return p.loaded(msg)
		}
	}
	return p, nil
}

// next moves to the next stop.
func (p Pane) next(c ui.Context) (ui.Pane, tea.Cmd) {
	switch {
	case p.stops == nil:
		p.seq++
		return p, tea.Batch(p.ask(c), notice("the roles have not loaded; asking again", false))
	case len(p.stops) == 1:
		return p, notice("one role only: "+p.stops[0], false)
	}
	p.at = (p.at + 1) % len(p.stops)
	return p, tea.Batch(p.arrive(), notice(fmt.Sprintf("reading as %s, %d of %d", p.stops[p.at], p.at+1, len(p.stops)), false))
}

// arrive starts the grid at the stop in force and keeps the selection.
func (p Pane) arrive() tea.Cmd {
	s := Arrival(p.Role(), p.viewer, p.ref)
	s.Select = p.selected
	return start(s)
}

// loaded takes tablo's answer about the viewer.
func (p Pane) loaded(msg ui.LoadedMsg) (ui.Pane, tea.Cmd) {
	v, ok := msg.Value.(view.Viewer)
	err := msg.Err
	if err == nil && !ok {
		err = fmt.Errorf("unexpected data of type %T", msg.Value)
	}
	if err != nil {
		if p.arrived {
			return p, nil
		}
		p.arrived = true
		return p, tea.Batch(p.arrive(), notice("roles: "+err.Error(), true))
	}
	stops := Stops(v.Roles)
	if p.stops == nil {
		p.stops, p.at, p.arrived = stops, 0, true
		return p, p.arrive()
	}
	was := p.stops[p.at]
	p.stops = stops
	if i := slices.Index(stops, was); i >= 0 {
		p.at = i
		return p, nil
	}
	p.at = 0
	return p, tea.Batch(p.arrive(), notice(fmt.Sprintf("the role %s no longer holds: reading as %s", was, stops[0]), false))
}
```

## Changed files

Each block is the whole of one file's change, as a unified diff against `main`.

### `internal/source/source.go`

```diff
--- a/internal/source/source.go
+++ b/internal/source/source.go
@@ -19,6 +19,8 @@
 	Window     int
 	Historical bool
 	Level      string // "detail" or "provenance"
+	Viewer     string // the email of the person at the keyboard; "" is nobody
+	Role       string // the role the viewer reads as; "" is the viewer's own
 }
 
 // Tableaux loads a tableau. A call blocks; the frame runs it in a command.
```

### `internal/source/file.go`

```diff
--- a/internal/source/file.go
+++ b/internal/source/file.go
@@ -46,5 +46,11 @@
 	if err != nil {
 		return view.Tableau{}, fmt.Errorf("%s: %w", f.Name(r), err)
 	}
+	if r.Viewer != "" {
+		t.Viewer = r.Viewer
+	}
+	if r.Role != "" {
+		t.Role = r.Role
+	}
 	return t, nil
 }
```

### `internal/ui/grid/grid.go`

```diff
--- a/internal/ui/grid/grid.go
+++ b/internal/ui/grid/grid.go
@@ -40,6 +40,12 @@
 	Open    []string
 	Select  string
 	Level   Level
+	// Mine takes the folds from the data: it unfolds every parent above a row
+	// that is the person's, and selects the first such row unless Select
+	// names a row. A row is the person's when it carries no label in a
+	// contextual tableau, and when the person acts in one of its cells in a
+	// global one. With no such row, Open decides.
+	Mine bool
 }
 
 // GlanceStart is the arrival of an observer and of the owner: the global
@@ -145,6 +151,9 @@
 
 // load runs one request off the event loop.
 func (g Grid) load(c ui.Context, seq uint64, req source.Request) tea.Cmd {
+	if req.View == "" {
+		return nil // a start that names no view waits for a StartMsg
+	}
 	src := g.src
 	return c.Load(ID, seq, func(ctx context.Context) (any, error) {
 		if src == nil {
@@ -242,7 +251,13 @@
 func (g Grid) arrive(s Start) Grid {
 	rows := g.data.Rows
 	g.open = map[string]bool{}
-	if s.Open == nil {
+	mine := -1
+	if s.Mine {
+		mine = g.unfoldMine()
+	}
+	if mine >= 0 {
+		// the data decides the folds
+	} else if s.Open == nil {
 		for _, r := range rows {
 			if r.Parent && r.Depth == 0 {
 				g.open[r.ID] = true
@@ -255,7 +270,7 @@
 	}
 	g.sel = ""
 	if len(rows) > 0 {
-		g.sel = rows[0].ID
+		g.sel = rows[max(mine, 0)].ID
 	}
 	if s.Select != "" && slices.ContainsFunc(rows, func(r view.Row) bool { return r.ID == s.Select }) {
 		g.sel = s.Select
@@ -263,6 +278,30 @@
 	return g
 }
 
+// unfoldMine unfolds every parent above a row that is the person's and returns
+// the index of the first such row, or -1 when the data holds none.
+func (g Grid) unfoldMine() int {
+	first := -1
+	var above []string // the ancestors of the row at hand, by depth
+	for i, r := range g.data.Rows {
+		above = above[:min(r.Depth, len(above))]
+		own := r.Label == ""
+		if g.data.View != "context" {
+			own = slices.ContainsFunc(r.Cells, func(c view.Cell) bool { return c.Acts })
+		}
+		if own {
+			if first < 0 {
+				first = i
+			}
+			for _, id := range above {
+				g.open[id] = true
+			}
+		}
+		above = append(above, r.ID)
+	}
+	return first
+}
+
 // survivor keeps the selected id when the new rows hold it, and otherwise
 // takes the nearest row above it, in the old order, that the new rows hold.
 func survivor(old, rows []view.Row, sel string) string {
```

### `cmd/tablotui/main.go`

```diff
--- a/cmd/tablotui/main.go
+++ b/cmd/tablotui/main.go
@@ -19,10 +19,12 @@
 
 	tea "charm.land/bubbletea/v2"
 
+	"github.com/nbyoung/tablotui/internal/identity"
 	"github.com/nbyoung/tablotui/internal/settings"
 	"github.com/nbyoung/tablotui/internal/source"
 	"github.com/nbyoung/tablotui/internal/ui"
 	"github.com/nbyoung/tablotui/internal/ui/grid"
+	"github.com/nbyoung/tablotui/internal/ui/role"
 )
 
 // version is the release version. GoReleaser sets it through
@@ -55,6 +57,7 @@
 	fs.SetOutput(io.Discard)
 	dir := fs.String("C", ".", "the project directory")
 	settingsPath := fs.String("settings", "", "the settings file (default: tablotui/settings.json in the user configuration directory)")
+	as := fs.String("as", "", "the email of the person at the keyboard (default: the Git identity)")
 	fixtures := fs.String("fixture", "", "draw from the tableau files in this directory")
 	showVersion := fs.Bool("version", false, "print the version and exit")
 	if err := checkHyphens(args, fs); err != nil {
@@ -99,12 +102,20 @@
 
 	ctx, cancel := context.WithCancel(context.Background())
 	defer cancel()
-	g := grid.New(grid.Options{
-		Source:   source.File{Dir: inDir(*dir, *fixtures)},
-		Settings: path,
-		Start:    grid.GlanceStart(),
-	})
-	opts := ui.Options{Panes: []ui.Pane{g}, Styles: ui.DefaultStyles(), Context: ctx}
+	email, err := identity.Email(ctx, *dir, *as)
+	if err != nil {
+		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
+		return exitUsage
+	}
+	src := source.File{Dir: inDir(*dir, *fixtures)}
+	g := grid.New(grid.Options{Source: src, Settings: path})
+	who := role.New(role.Options{Source: src, Viewer: email})
+	opts := ui.Options{
+		Panes:    []ui.Pane{g, who},
+		Commands: []ui.Command{role.Command()},
+		Styles:   ui.DefaultStyles(),
+		Context:  ctx,
+	}
 	if err := ui.CheckKeys(opts); err != nil {
 		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
 		return exitFailure
@@ -154,7 +165,7 @@
 
 // usage prints each long option with its double hyphen.
 func usage(fs *flag.FlagSet, w io.Writer) {
-	_, _ = fmt.Fprintln(w, "usage: tablotui [-C <dir>] [--settings <file>] [--fixture <dir>] [--version] [--help]")
+	_, _ = fmt.Fprintln(w, "usage: tablotui [-C <dir>] [--as <email>] [--settings <file>] [--fixture <dir>] [--version] [--help]")
 	_, _ = fmt.Fprintln(w, "options:")
 	fs.VisitAll(func(f *flag.Flag) {
 		name := "--" + f.Name
```

### `internal/view/tableau_test.go`

```diff
--- a/internal/view/tableau_test.go
+++ b/internal/view/tableau_test.go
@@ -9,7 +9,7 @@
 
 func TestDecodeFixtures(t *testing.T) {
 	files, err := filepath.Glob("../ui/grid/testdata/*.json")
-	if err != nil || len(files) != 5 {
+	if err != nil || len(files) != 6 {
 		t.Fatalf("fixtures = %v, %v", files, err)
 	}
 	for _, f := range files {
```

## The test spy

The role tests reach the fixtures through this source, which records each request, scripts the roles and can fail. It lives in the test file of `internal/ui/role`.

```go
// spy records every request and can fail or script the viewer.
type spy struct {
	source.File
	reqs    []source.Request
	asked   int
	roles   []string
	failing bool
	lenient bool // serve the window 1 file whatever the window and the switch
}

func (s *spy) Tableau(ctx context.Context, r source.Request) (view.Tableau, error) {
	s.reqs = append(s.reqs, r)
	if s.lenient {
		r.Window, r.Historical = 1, false
	}
	return s.File.Tableau(ctx, r)
}

func (s *spy) Viewer(ctx context.Context, ref, email string) (view.Viewer, error) {
	s.asked++
	if s.failing {
		return view.Viewer{}, errors.New("the project does not load")
	}
	if s.roles != nil {
		return view.Viewer{Email: email, Roles: s.roles}, nil
	}
	if roles, ok := held[email]; ok {
		return view.Viewer{Email: email, Roles: roles}, nil
	}
	return s.File.Viewer(ctx, ref, email)
}

// held is the roles the two emails of the Tableaux tooling plan hold, as
// PLAN.md states them under Roles.
var held = map[string][]string{
	"nbyoung@nbyoung.com":   {"owner", "authority", "assignee", "reviewer"},
	"noreply@anthropic.com": {"authority", "assignee", "contributor", "agent"},
}
```

The tests build the frame as the command does, with `fixtures` set to `../grid/testdata`:

```go
src := &spy{File: source.File{Dir: fixtures}}
g := grid.New(grid.Options{Source: src, Settings: settingsPath})
who := New(Options{Source: src, Viewer: viewer})
o := ui.Options{Panes: []ui.Pane{g, who}, Commands: []ui.Command{Command()}}
```
