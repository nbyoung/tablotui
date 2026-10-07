package grid

import (
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/view"
)

// ruleAt returns the line of the strip's rule, or -1.
func ruleAt(r *rig) int {
	for i, l := range r.content() {
		if i > 2 && strings.HasPrefix(l, "──────────") {
			return i
		}
	}
	return -1
}

// T10: the strip.
func TestStripLevels(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 30)
	// glance: no strip, so 26 body lines hold the header, the rule and 24 rows.
	if ruleAt(r) != -1 {
		t.Error("glance draws a strip")
	}
	rowsAt := func() int { a, b, _ := shownRange(t, r); return b - a + 1 }
	if got := rowsAt(); got != 6 {
		t.Errorf("rows = %d", got)
	}
	r.keys("E")
	if got := rowsAt(); got != 25 {
		t.Errorf("glance rows = %d, want 25", got)
	}
	r.keys("d")
	if got := rowsAt(); got != 20 || ruleAt(r) != 23 {
		t.Errorf("detail: %d rows, rule at %d, want 20 and 23", got, ruleAt(r))
	}
	r.keys("p")
	if got := rowsAt(); got != 15 || ruleAt(r) != 18 {
		t.Errorf("provenance: %d rows, rule at %d, want 15 and 18", got, ruleAt(r))
	}
	r.keys("p")
	if got := rowsAt(); got != 20 {
		t.Errorf("p at provenance returns to detail: %d rows", got)
	}
	r.keys("d")
	if got := rowsAt(); got != 25 || ruleAt(r) != -1 {
		t.Errorf("d at detail closes the strip: %d rows", got)
	}
	r.keys("p")
	if got := rowsAt(); got != 15 {
		t.Errorf("p at glance opens provenance: %d rows", got)
	}
	r.keys("d")
	if got := rowsAt(); got != 25 {
		t.Errorf("d at provenance closes the strip: %d rows", got)
	}
}

func TestStripShrinksWhenRowsRunShort(t *testing.T) {
	// With every row unfolded, a strip that would leave fewer than 3 rows gives
	// way to the next smaller one.
	cases := []struct {
		h     int
		level string // the key that opens it: d for detail, p for provenance
		rows  int
	}{
		{h: 100, level: "p", rows: 37}, // room for the whole table
		{h: 24, level: "p", rows: 9},   // 21 lines: 2 + 10 + 9
		{h: 18, level: "p", rows: 3},   // 15 lines: 2 + 10 + 3
		{h: 17, level: "p", rows: 7},   // 14 lines: provenance would leave 2, so detail: 2 + 5 + 7
		{h: 13, level: "p", rows: 3},   // 10 lines: detail leaves 3
		{h: 12, level: "p", rows: 7},   // 9 lines: detail would leave 2, so none: 2 + 7
		{h: 13, level: "d", rows: 3},
		{h: 12, level: "d", rows: 7},
		{h: 8, level: "p", rows: 3}, // 5 lines: no strip
	}
	for _, c := range cases {
		r := newRig(t, fixtures(), "", GlanceStart(), 100, c.h).keys("E " + c.level)
		a, b, _ := shownRange(t, r)
		if got := b - a + 1; got != c.rows {
			t.Errorf("height %d, key %s: %d rows, want %d", c.h, c.level, got, c.rows)
		}
		if lines := r.content(); len(lines) != c.h {
			t.Errorf("height %d: %d lines", c.h, len(lines))
		}
	}
}

func TestStripContent(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("E d down down")
	lines := r.content()
	at := ruleAt(r)
	for i, want := range []string{
		"c2ad Roles · 🧱 implementation 🟢 nominal · 2026-09-30",
		"The Roles text is on the trunk and covers all seven roles; the validate gate awaits the owner",
		"",
		"",
	} {
		if got := strings.TrimRight(stripANSI(lines[at+1+i]), " "); got != want {
			t.Errorf("strip line %d = %q, want %q", i+1, got, want)
		}
	}
	// A rolled-up parent names the leaf.
	r2 := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("d")
	if got := r2.line(ruleAt(r2) + 4); got != "rolled up from 99f0 Gate definition view in Markdown" {
		t.Errorf("rolled-up line = %q", got)
	}
	// A subproject names its snapshot.
	r3 := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("down down down down d")
	if got := r3.line(ruleAt(r3) + 4); got != "snapshot of 40e8 tablotui at pin a2878b5, 🔧 function there" {
		t.Errorf("snapshot line = %q", got)
	}
	// A folded column gives its counts gate by gate.
	r4 := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("d right")
	if got := r4.line(ruleAt(r4) + 4); got != "folded: ❔ undefined 0" {
		t.Errorf("folded line = %q", got)
	}
}

func TestLongNoteEndsWithAnEllipsis(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("down down down down d")
	at := ruleAt(r)
	if got := r.line(at + 2); !strings.HasSuffix(got, "collapse,") || len([]rune(got)) > 100 {
		t.Errorf("first note line = %q", got)
	}
	if got := r.line(at + 3); !strings.HasSuffix(got, " …") {
		t.Errorf("second note line = %q, want it to end ' …'", got)
	}
}

func TestProvenanceFacts(t *testing.T) {
	// Five facts draw three and "… 2 more".
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("p right right")
	at := ruleAt(r)
	want := []string{
		"provenance of 437e at 📝 defined",
		"  Status: derived, never stored",
		"  Rolls up from: bc63 Method, then 2034 Views, then bc86 Markdown views, then 99f0 Gate definition …",
		"  Why: the first in display order of 5 children that tie",
		"  … 2 more",
	}
	for i, w := range want {
		if got := r.line(at + 5 + i); got != w {
			t.Errorf("provenance line %d = %q, want %q", i, got, w)
		}
	}
	// Four facts draw all four.
	r = newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("E p down down right right right right right right")
	at = ruleAt(r)
	if got := r.line(at + 9); !strings.HasPrefix(got, "  Command: git log") {
		t.Errorf("fourth fact = %q", got)
	}
	// A cell with no fact, and a cursor off the gate cells.
	r.keys("right")
	if got := r.line(at + 5); got != "this cell holds no fact" {
		t.Errorf("no-fact line = %q", got)
	}
	r.keys("left left left left left left left")
	if got := r.line(at + 5); got != "move the column cursor to a gate cell" {
		t.Errorf("off-cell line = %q", got)
	}
	r.keys("right")
	if got := r.line(at + 5); got != "move the column cursor to a gate cell" {
		t.Errorf("folded-column line = %q", got)
	}
}

// T10: p asks for provenance once.
func TestProvenanceIsAskedOnce(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 30)
	if len(src.reqs) != 1 || src.reqs[0].Level != "detail" {
		t.Fatalf("requests = %+v", src.reqs)
	}
	r.keys("p")
	if len(src.reqs) != 2 || src.last().Level != "provenance" {
		t.Fatalf("p did not ask for provenance: %+v", src.reqs)
	}
	r.keys("p p p d p")
	if len(src.reqs) != 2 {
		t.Errorf("later p keys asked again: %+v", src.reqs)
	}
	// The request keeps asking for provenance afterwards.
	r.keys("w")
	if src.last().Level != "provenance" {
		t.Errorf("a later request = %+v", src.last())
	}
}

func TestStripNeedsASelection(t *testing.T) {
	src := &recorder{fn: func(source.Request) (view.Tableau, error) {
		tb := load(t, "tableau-empty.json")
		return tb, nil
	}}
	r := newRig(t, src, "", GlanceStart(), 100, 30).keys("d p right x s")
	if len(r.content()) != 30 {
		t.Errorf("%d lines", len(r.content()))
	}
}
