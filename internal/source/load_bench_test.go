package source

import (
	"errors"
	"testing"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/watch"
)

// benchCommit is the umbrella's commit 171b's A8 times the load at.
const benchCommit = "50a373b2857a36e4ef1a754605433943d6bfe324"

// BenchmarkLoad times 171b's A8: one in-process load of the umbrella's plan
// at 50a373b, read from the working tree as the live view reads it by
// default, with the views the frame shows. Each iteration runs the live
// refresh's Loader, Tablo.Load, which is Repository.Load and the check that
// the project gives view data; stores the snapshot; and derives what the
// frame then asks of it: the owner's roles and arrival, the tableau at a
// glance, and with the four panes open on the root, the task, the queue, the
// blockage and the history. A8 bounds it at 100 ms.
//
//	TABLEAUX_DIR=…/tableaux go test ./internal/source -run '^$' -bench BenchmarkLoad -benchtime 20x -cpuprofile cpu.out
func BenchmarkLoad(b *testing.B) {
	dir := corpustest.Umbrella(b, benchCommit)
	const owner = "nbyoung@nbyoung.com"
	for _, c := range []struct {
		name  string
		panes bool
	}{{"arrival", false}, {"four panes", true}} {
		b.Run(c.name, func(b *testing.B) {
			src, err := NewTablo(b.Context(), dir, "", nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := b.Context()
			var diag *DiagnosticError
			for b.Loop() {
				snap, err := src.Load(ctx, watch.State{})
				if err != nil && !errors.As(err, &diag) {
					b.Fatal(err)
				}
				src.Store(snap)
				v, err := src.Viewer(ctx, "", owner)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := src.Tableau(ctx, Request{View: "tableau", Window: 1, Level: "detail", Viewer: owner, Role: v.Roles[0]}); err != nil && !errors.As(err, &diag) {
					b.Fatal(err)
				}
				if !c.panes {
					continue
				}
				r := ListRequest{Task: "437e", Viewer: owner, Role: v.Roles[0]}
				if _, err := src.Task(ctx, r); err != nil {
					b.Fatal(err)
				}
				if _, err := src.Queue(ctx, r); err != nil {
					b.Fatal(err)
				}
				if _, err := src.Blockage(ctx, r); err != nil {
					b.Fatal(err)
				}
				if _, err := src.History(ctx, r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
