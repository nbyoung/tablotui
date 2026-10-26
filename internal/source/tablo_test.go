package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/watch"
)

// poller records each list of checkouts the adapter names.
type poller struct{ lists [][]string }

func (p *poller) SetSubmodules(dirs []string) { p.lists = append(p.lists, dirs) }

// stored opens the adapter on dir at ref, loads the state st and stores the
// snapshot.
func stored(t *testing.T, dir, ref string, st watch.State) (*Tablo, *poller) {
	t.Helper()
	p := &poller{}
	src, err := NewTablo(t.Context(), dir, ref, p)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := src.Load(t.Context(), st)
	var diag *DiagnosticError
	if err != nil && !errors.As(err, &diag) { // a diagnostic beside the snapshot leaves it good
		t.Fatal(err)
	}
	src.Store(snap)
	return src, p
}

// The adapter maps each field of a Request to tablo's parameters, and tablo
// echoes them in the envelope's params and viewer.
func TestTabloRequest(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	src, p := stored(t, dir, "", watch.State{})
	if len(p.lists) != 1 || len(p.lists[0]) != 1 || p.lists[0][0] != filepath.Join(dir, "firmware") {
		t.Errorf("the checkouts named = %v, want the firmware checkout once", p.lists)
	}
	ctx := t.Context()
	tb, err := src.Tableau(ctx, Request{View: "tableau", Window: 1, Level: "detail"})
	if err != nil {
		t.Fatal(err)
	}
	if tb.Command != "tableau" || tb.Project.Title != "Weather station" || tb.Source.Ref != "main" || len(tb.Rows) != 6 ||
		tb.Params.Window == nil || *tb.Params.Window != 1 || tb.Params.Level != "detail" || tb.Viewer.Email != "" {
		t.Errorf("the tableau: %+v %+v %+v, %d rows", tb.Source, tb.Project, tb.Params, len(tb.Rows))
	}
	cases := []struct {
		name  string
		r     Request
		check func(t *testing.T, r Request)
	}{
		{"person and viewer", Request{View: "context", Person: "ben@example.org", Window: 1, Level: "detail", Viewer: "ada@example.org", Role: "owner"}, nil},
		{"task", Request{View: "context", Task: "4e2b", Window: 1, Level: "provenance", Viewer: "ben@example.org", Role: "reviewer"}, nil},
		{"window and historical", Request{View: "tableau", Window: 0, Historical: true, Level: "detail", Person: "ben@example.org"}, nil},
		{"no view is the tableau", Request{Window: 2, Level: "detail"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb, err := src.Tableau(ctx, c.r)
			if err != nil {
				t.Fatal(err)
			}
			want := c.r.View
			if want == "" {
				want = "tableau"
			}
			if tb.Command != want || tb.Params.Person != c.r.Person || tb.Params.Task != c.r.Task || tb.Params.Historical != c.r.Historical ||
				tb.Params.Window == nil || *tb.Params.Window != c.r.Window || tb.Params.Level != c.r.Level ||
				tb.Params.Role != c.r.Role || tb.Viewer.Email != c.r.Viewer {
				t.Errorf("%s: params %+v, viewer %+v", tb.Command, tb.Params, tb.Viewer)
			}
		})
	}
	var nf *tablo.NotFoundError
	if _, err := src.Tableau(ctx, Request{View: "context", Task: "ffff", Window: 1, Level: "detail"}); !errors.As(err, &nf) || nf.Name != "ffff" {
		t.Errorf("an unknown task gives %v", err)
	}
	var ue *tablo.UsageError
	if _, err := src.Tableau(ctx, Request{View: "context", Window: 1, Level: "detail"}); !errors.As(err, &ue) {
		t.Errorf("a context with no task, person or viewer gives %v", err)
	}
}

// The adapter maps each field of a ListRequest, the queue's person is the
// viewer, and the roles come from the stored snapshot.
func TestTabloListsAndViewers(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	src, _ := stored(t, dir, "", watch.State{})
	ctx := t.Context()
	task, err := src.Task(ctx, ListRequest{View: "task", Task: "3c5d", Level: "detail"})
	if err != nil || task.Task.ID != "3c5d" || task.Level.String() != "detail" {
		t.Errorf("task: %+v, %v", task.Task, err)
	}
	q, err := src.Queue(ctx, ListRequest{View: "queue", Viewer: "ada@example.org", Role: "owner"})
	if err != nil || q.Params.Person != "ada@example.org" {
		t.Errorf("queue: %+v, %v", q.Params, err)
	}
	if _, err := src.Queue(ctx, ListRequest{View: "queue"}); !errors.Is(err, ErrNoPerson) {
		t.Errorf("a queue with no viewer gives %v", err)
	}
	b, err := src.Blockage(ctx, ListRequest{View: "blockage", Task: "4e2b", Level: "provenance"})
	if err != nil || b.Params.Task != "4e2b" || b.Level.String() != "provenance" {
		t.Errorf("blockage: %+v, %v", b.Params, err)
	}
	h, err := src.History(ctx, ListRequest{View: "history", Task: "c07d", Viewer: "ben@example.org"})
	if err != nil || h.Subject.ID != "c07d" {
		t.Errorf("history: %+v, %v", h.Subject, err)
	}
	var nf *tablo.NotFoundError
	if _, err := src.Task(ctx, ListRequest{View: "task", Task: "ffff"}); !errors.As(err, &nf) {
		t.Errorf("an unknown task gives %v", err)
	}
	for _, c := range []struct{ email, want string }{
		{"ada@example.org", "owner authority assignee contributor"},
		{"ben@example.org", "authority assignee contributor reviewer"},
		{"eve@example.org", "observer"},
		{"", "observer"},
	} {
		v, err := src.Viewer(ctx, "", c.email)
		if err != nil || v.Email != c.email || strings.Join(v.Roles, " ") != c.want {
			t.Errorf("Viewer(%q) = %+v, %v", c.email, v, err)
		}
	}
}

// A call before the first snapshot waits for it, and a cancelled call
// returns the context's error.
func TestTabloWaitsForTheFirstSnapshot(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	src, err := NewTablo(t.Context(), dir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := src.Viewer(cancelled, "", "ada@example.org"); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled call gives %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := src.Tableau(t.Context(), Request{View: "tableau", Window: 1, Level: "detail"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("the call returned before the first snapshot: %v", err)
	default:
	}
	snap, err := src.Load(t.Context(), watch.State{})
	if err != nil {
		t.Fatal(err)
	}
	src.Store(snap)
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Minute):
		t.Fatal("the call still waits after the first snapshot")
	}
	src.Store(nil) // a nil snapshot changes nothing
	if _, err := src.Viewer(t.Context(), "", ""); err != nil {
		t.Error(err)
	}
}

// At a ref, the Loader reads the commit of the state and the envelope names
// the ref.
func TestTabloAtARef(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	labels := corpustest.Labels(t, "weather-station")
	src, p := stored(t, dir, "main", watch.State{Commit: labels["W1"]})
	tb, err := src.Tableau(t.Context(), Request{View: "tableau", Window: 1, Level: "detail"})
	if err != nil {
		t.Fatal(err)
	}
	if tb.Source.Ref != "main" || tb.Source.Commit != labels["W1"] || len(p.lists) != 1 || len(p.lists[0]) != 0 {
		t.Errorf("source %+v, checkouts %v", tb.Source, p.lists)
	}
	var nf *tablo.NotFoundError
	if _, err := src.Load(t.Context(), watch.State{Commit: strings.Repeat("0", 40)}); !errors.As(err, &nf) {
		t.Errorf("a commit the repository lacks gives %v", err)
	}
}

// Decision 16: an error diagnostic beside view data gives the view and a
// *DiagnosticError; only a missing or unaccepted version, an unusable
// gates.yaml or a broken tree fails the load, with its first diagnostic.
func TestTabloDiagnostics(t *testing.T) {
	dir := corpustest.Entry(t, "status-unknown-state")
	src, _ := stored(t, dir, "", watch.State{})
	tb, err := src.Tableau(t.Context(), Request{View: "tableau", Window: 1, Level: "detail"})
	var diag *DiagnosticError
	if !errors.As(err, &diag) || len(tb.Rows) == 0 || diag.Diagnostics[0].Code != "S6" {
		t.Fatalf("%d rows beside %v", len(tb.Rows), err)
	}
	if got := diag.Error(); !strings.HasPrefix(got, ".tableaux/status/") || !strings.Contains(got, ": S6 ") {
		t.Errorf("the diagnostic reads %q", got)
	}
	if _, err := src.Task(t.Context(), ListRequest{View: "task", Task: tb.Rows[0].ID, Level: "detail"}); err != nil {
		t.Errorf("a list beside the diagnostic: %v", err)
	}
	// Load derives the Gates envelope: its snapshot comes beside the same
	// error diagnostics as the tableau's (171b decision 15).
	snap, err := src.Load(t.Context(), watch.State{})
	var loadDiag *DiagnosticError
	if snap == nil || !errors.As(err, &loadDiag) || loadDiag.Error() != diag.Error() || !reflect.DeepEqual(loadDiag.Diagnostics, diag.Diagnostics) {
		t.Errorf("Load gives %v, %v; want the snapshot beside %v", snap, err, diag)
	}
	for entry, code := range map[string]string{
		"version-missing":        "P2",
		"version-major-mismatch": "P4",
		"gates-missing":          "G1",
		"tree-parent-cycle":      "T10",
	} {
		t.Run(entry, func(t *testing.T) {
			src, err := NewTablo(t.Context(), corpustest.Entry(t, entry), "", nil)
			if err != nil {
				t.Fatal(err)
			}
			snap, err := src.Load(t.Context(), watch.State{})
			if err == nil || snap != nil || !strings.Contains(err.Error(), ": "+code+" ") && !strings.HasPrefix(err.Error(), code+" ") {
				t.Errorf("the load gives %v, %v; want the first diagnostic, %s", snap, err, code)
			}
			if errors.As(err, &diag) {
				t.Error("a failed load is a *DiagnosticError")
			}
		})
	}
}

// NewTablo outside a repository is a *tablo.ReadError.
func TestNewTabloOutsideARepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".tableaux"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", dir)
	var re *tablo.ReadError
	if _, err := NewTablo(t.Context(), dir, "", nil); !errors.As(err, &re) {
		t.Errorf("NewTablo gives %v", err)
	}
}

func TestDiagnosticText(t *testing.T) {
	cases := []struct {
		d    Diagnostic
		want string
	}{
		{Diagnostic{Code: "S6", Path: ".tableaux/status/a1c0.yaml", Line: 2, Col: 8, Message: "no such state"}, ".tableaux/status/a1c0.yaml:2:8: S6 no such state"},
		{Diagnostic{Code: "P2", Path: ".tableaux/version.yaml", Message: "missing"}, ".tableaux/version.yaml: P2 missing"},
		{Diagnostic{Code: "S11", Path: ".tableaux/status/e3cb.yaml", Line: 1, Message: "no review"}, ".tableaux/status/e3cb.yaml:1: S11 no review"},
		{Diagnostic{Code: "T10", Message: "a cycle"}, "T10 a cycle"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("%+v reads %q, want %q", c.d, got, c.want)
		}
	}
	e := &DiagnosticError{Diagnostics: []Diagnostic{cases[0].d, cases[1].d, cases[3].d}}
	if got, want := e.Error(), cases[0].want+" (and 2 more)"; got != want {
		t.Errorf("the error reads %q, want %q", got, want)
	}
	if got := (&DiagnosticError{}).Error(); got != "the project has errors" {
		t.Errorf("no diagnostic reads %q", got)
	}
	if !slices.Equal(errorsOf([]tablo.Diagnostic{{Severity: "warning", Code: "R9"}, {Severity: "error", Code: "S6"}}), []Diagnostic{{Severity: "error", Code: "S6"}}) {
		t.Error("errorsOf keeps what is no error")
	}
}
