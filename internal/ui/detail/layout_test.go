package detail

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/ui"
)

var paneIDs = []string{"grid", TaskID, QueueID, BlockageID, HistoryID}

// T4: the layout tiles the body.
func TestLayoutTiles(t *testing.T) {
	var widths, heights []int
	for w := 40; w <= 260; w += 7 {
		widths = append(widths, w)
	}
	for h := 5; h <= 60; h += 3 {
		heights = append(heights, h)
	}
	widths = append(widths, 119, 120, 121, 160, 240, 260)
	heights = append(heights, 11, 12, 13, 60)
	for _, zoom := range []bool{false, true} {
		a := &arrangement{zoom: zoom}
		for _, w := range widths {
			for _, h := range heights {
				for n := 0; n <= 4; n++ {
					open := paneIDs[:n+1]
					for _, focus := range open {
						rects := a.layout(w, h, open, focus)
						tag := fmt.Sprintf("%dx%d, %d panes, focus %s, zoom %v", w, h, n, focus, zoom)
						checkTiling(t, tag, w, h, rects)
						if !hasRect(rects, focus) {
							t.Errorf("%s: the focus has no rectangle: %+v", tag, rects)
						}
						if zoom || n == 0 {
							continue
						}
						if len(rects) > 1 {
							for _, r := range rects[1:] {
								if r.W < paneWidth || r.H < paneHeight {
									t.Errorf("%s: pane %s is %dx%d", tag, r.Pane, r.W, r.H)
								}
							}
						}
					}
				}
			}
		}
	}
}

func hasRect(rects []ui.Rect, id string) bool {
	for _, r := range rects {
		if r.Pane == id {
			return true
		}
	}
	return false
}

// checkTiling says that the rectangles lie inside the body, do not overlap
// and cover it.
func checkTiling(t *testing.T, tag string, w, h int, rects []ui.Rect) {
	t.Helper()
	cells := make([]int, w*h)
	for _, r := range rects {
		if r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > w || r.Y+r.H > h {
			t.Errorf("%s: %+v lies outside the body", tag, r)
			return
		}
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				cells[y*w+x]++
			}
		}
	}
	for i, n := range cells {
		if n != 1 {
			t.Errorf("%s: cell %d,%d is covered %d times: %+v", tag, i%w, i/w, n, rects)
			return
		}
	}
}

func TestLayoutWithNoPane(t *testing.T) {
	if got := (&arrangement{}).layout(80, 24, nil, ""); got != nil {
		t.Errorf("layout of nothing = %+v", got)
	}
}

func TestLayoutLeavesOutAPaneTheRegionDoesNotShow(t *testing.T) {
	a := &arrangement{}
	// 80x11 is the single form: the grid has the body while it has the focus.
	got := a.layout(80, 11, paneIDs[:3], "grid")
	if len(got) != 1 || got[0].Pane != "grid" {
		t.Errorf("single form with the grid focused = %+v", got)
	}
	// A region that holds two panes shows the last two and moves for the focus.
	got = a.layout(80, 15, paneIDs, "task")
	var ids []string
	for _, r := range got[1:] {
		ids = append(ids, r.Pane)
	}
	if strings.Join(ids, " ") != "task queue blockage" && strings.Join(ids, " ") != "task queue" {
		t.Errorf("shown panes = %v", ids)
	}
	if !hasRect(got, "task") {
		t.Errorf("the focused pane is not shown: %+v", got)
	}
}

// layout.txt fixes thirteen cases. A line reads
//
//	<w>x<h> <open panes> focus <pane>: <pane> <x>,<y> <w>x<h>; ...
func TestLayoutCases(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(golden(t, "layout.txt")), "\n")
	if len(lines) != 13 {
		t.Fatalf("layout.txt holds %d cases, want 13", len(lines))
	}
	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			head, rest, ok := strings.Cut(line, ": ")
			if !ok {
				t.Fatalf("no colon in %q", line)
			}
			var w, h int
			var panes, focus string
			fields := strings.Fields(head)
			if len(fields) != 4 || fields[2] != "focus" {
				t.Fatalf("head %q", head)
			}
			if _, err := fmt.Sscanf(fields[0], "%dx%d", &w, &h); err != nil {
				t.Fatal(err)
			}
			panes, focus = fields[1], fields[3]
			var got []string
			for _, r := range (&arrangement{}).layout(w, h, strings.Split(panes, ","), focus) {
				got = append(got, fmt.Sprintf("%s %d,%d %dx%d;", r.Pane, r.X, r.Y, r.W, r.H))
			}
			if s := strings.Join(got, " "); s != rest {
				t.Errorf("layout = %q, want %q", s, rest)
			}
		})
	}
}
