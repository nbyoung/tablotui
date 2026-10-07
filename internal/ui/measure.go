package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Measure counts terminal cells the way the terminal in use draws them. The
// zero value counts by code point, as a terminal without grapheme clustering
// does. No pane pads through Lip Gloss, which always measures by grapheme: a
// pane pads with Measure and applies a style to the padded text.
type Measure struct{ Method ansi.Method }

// vs16 is the variation selector that asks for emoji presentation.
const vs16 = '️'

// Width returns the cells of Clean(s).
func (m Measure) Width(s string) int { return m.Method.StringWidth(m.Clean(s)) }

// Clean returns s as the terminal draws it. Under ansi.WcWidth it removes each
// U+FE0F that follows a code point one cell wide, so a symbol such as the gear
// draws as the one-cell text glyph that every terminal agrees on. Under
// ansi.GraphemeWidth it returns s as it stands.
func (m Measure) Clean(s string) string {
	if m.Method != ansi.WcWidth || !strings.ContainsRune(s, vs16) {
		return s
	}
	var b strings.Builder
	var prev rune
	for i, r := range s {
		if r == vs16 && i > 0 && ansi.WcWidth.StringWidth(string(prev)) == 1 {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// Fit cuts s with "…" to w cells and pads it on the right to exactly w cells.
func (m Measure) Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = m.Clean(s)
	n := m.Method.StringWidth(s)
	if n > w {
		s = m.Method.Truncate(s, w, "…")
		n = m.Method.StringWidth(s)
	}
	return s + strings.Repeat(" ", max(0, w-n))
}

// Centre pads s on both sides to exactly w cells, with the odd cell on the
// right. It cuts s with "…" when s is wider than w.
func (m Measure) Centre(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = m.Clean(s)
	n := m.Method.StringWidth(s)
	if n > w {
		return m.Fit(s, w)
	}
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}

// Wrap fills each of n lines with whole words, the words split at spaces, and
// returns exactly n lines of at most w cells; the lines left over are empty.
// When words remain after line n, it drops words from that line until " …"
// fits after them.
func (m Measure) Wrap(s string, w, n int) []string {
	lines := make([]string, max(n, 0))
	if n <= 0 || w <= 0 {
		return lines
	}
	words := strings.Fields(m.Clean(s))
	line := 0
	var cur []string
	curW := 0
	flush := func() {
		lines[line] = strings.Join(cur, " ")
		cur, curW = nil, 0
		line++
	}
	i := 0
	for ; i < len(words); i++ {
		word := words[i]
		ww := m.Method.StringWidth(word)
		if ww > w {
			word = m.Method.Truncate(word, w, "…")
			ww = m.Method.StringWidth(word)
		}
		if len(cur) > 0 && curW+1+ww > w {
			if line == n-1 {
				break
			}
			flush()
		}
		if len(cur) > 0 {
			curW++
		}
		cur = append(cur, word)
		curW += ww
	}
	if i < len(words) {
		// Words remain after the last line: end it with " …".
		for len(cur) > 0 && curW+2 > w {
			last := cur[len(cur)-1]
			cur = cur[:len(cur)-1]
			curW -= m.Method.StringWidth(last)
			if len(cur) > 0 {
				curW--
			}
		}
		lines[line] = strings.Join(cur, " ") + " …"
		if len(cur) == 0 {
			lines[line] = "…"
		}
		return lines
	}
	if len(cur) > 0 && line < n {
		lines[line] = strings.Join(cur, " ")
	}
	return lines
}
