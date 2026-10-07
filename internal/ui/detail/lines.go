package detail

import (
	"strings"

	"github.com/nbyoung/tablotui/internal/ui"
)

// mark stands in the indent of a line that names the task the grid selects.
const mark = ">"

// Line is one logical line of a pane's document. A builder returns lines; the
// pane wraps them to its width and draws the rows.
type Line struct {
	Label  string // drawn first and padded to Hang cells; "" for none
	Text   string // wraps at single spaces
	Indent int    // cells before the label or the text
	Hang   int    // cells before each continuation row, after Indent
	Head   bool   // a heading: draws in Styles.Header
	Marked bool   // the line names the task the grid selects: ">" stands in its indent
	Item   int    // the item of the view the line belongs to, from 0; -1 for none
	Task   string // the task enter selects in the grid; "" for none
	Gate   string // the gate enter moves the grid's column cursor to; "" for none
}

// row is one screen row of a wrapped line.
type row struct {
	text string
	line int // the index of its Line
	sub  int // its position within that line, from 0
}

// wrap breaks every line into rows of at most w cells.
func wrap(lines []Line, w int, m ui.Measure) []row {
	var out []row
	for i, l := range lines {
		for k, t := range wrapLine(l, w, m) {
			out = append(out, row{text: t, line: i, sub: k})
		}
	}
	return out
}

// wrapLine breaks one line. The indent never exceeds a quarter of the width and
// the hang a half, so every row keeps a quarter of it for its text.
func wrapLine(l Line, w int, m ui.Measure) []string {
	if w <= 0 {
		return []string{""}
	}
	indent := min(l.Indent, w/4)
	hang := min(l.Hang, w/2)
	first := strings.Repeat(" ", indent)
	room := w - indent
	if l.Label != "" {
		first += m.Fit(l.Label, hang)
		room -= hang
	}
	rest := strings.Repeat(" ", indent+hang)
	restRoom := w - indent - hang

	var rows []string
	cur, curW := "", 0
	prefix, limit := first, room
	flush := func() {
		rows = append(rows, strings.TrimRight(prefix+cur, " "))
		cur, curW = "", 0
		prefix, limit = rest, restRoom
	}
	for _, word := range strings.Fields(m.Clean(l.Text)) {
		ww := m.Method.StringWidth(word)
		if curW > 0 && curW+1+ww > limit {
			flush()
		}
		for ww > limit {
			if curW > 0 {
				flush()
			}
			head := m.Method.Truncate(word, limit, "")
			if head == "" {
				break
			}
			cur, curW = head, m.Method.StringWidth(head)
			flush()
			word = word[len(head):]
			ww = m.Method.StringWidth(word)
		}
		if curW > 0 {
			cur += " "
			curW++
		}
		cur += word
		curW += ww
	}
	if curW > 0 || len(rows) == 0 {
		flush()
	}
	if l.Marked && indent >= 2 {
		rows[0] = mark + strings.Repeat(" ", indent-1) + strings.TrimPrefix(rows[0], strings.Repeat(" ", indent))
	}
	return rows
}
