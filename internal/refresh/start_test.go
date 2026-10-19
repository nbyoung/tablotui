package refresh_test

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/refresh"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/watch"
	"github.com/nbyoung/tablotui/internal/watch/watchtest"
)

// A live start on a project with a submodule loads once: the first load names
// the checkout of weather-station's firmware through SetSubmodules, and the
// watcher then waits an interval, with no settle and no second load. The test
// clock releases every wait, so the test sleeps on no wall clock. It skips
// without TABLEAUX_CORPUS.
func TestAStartWithASubmoduleLoadsOnce(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src, err := watch.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	w := watch.New(src)
	clk := watchtest.NewClock()
	w.Clock = clk
	tab, err := source.NewTablo(ctx, src.Dir, "", w)
	if err != nil {
		t.Fatal(err)
	}
	m := refresh.New(ctx, src, w, time.Now, tab.Load)

	// The first state, and the one load it starts.
	msg := m.Init()()
	m, cmd := m.Update(msg)
	ld, ok := cmd().(refresh.Loaded[*tablo.Snapshot])
	if !ok || ld.Err != nil || ld.State.Modules != "" {
		t.Fatalf("the first load %+v", ld)
	}
	if len(ld.Data.Checkouts()) == 0 {
		t.Fatal("the load names no submodule checkout")
	}
	m, cmd = m.Update(ld)
	if !m.Status().Good {
		t.Fatalf("status %+v", m.Status())
	}

	// The wait for the next change: two intervals pass, and nothing loads.
	next := make(chan tea.Msg, 1)
	go func() { next <- cmd() }()
	for range 2 {
		if d := <-clk.Asked; d != w.Interval {
			t.Fatalf("the watcher waits %v after the first load, want the interval %v", d, w.Interval)
		}
		select {
		case got := <-next:
			t.Fatalf("a second load starts: %+v", got)
		default:
		}
		clk.Tick <- time.Time{}
	}
	if d := <-clk.Asked; d != w.Interval {
		t.Fatalf("the watcher waits %v, want the interval %v", d, w.Interval)
	}
	cancel()
	if got := <-next; got != nil {
		t.Fatalf("the wait ends with %+v", got)
	}
}
