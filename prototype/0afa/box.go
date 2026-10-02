package main

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fit clips s to w display cells and pads it with spaces to exactly w.
// ansi.Truncate drops a wide symbol that does not fit whole; it never splits it.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

// wrap breaks each logical line at w cells and returns the display lines.
func wrap(c Content, w int) []string {
	var out []string
	for _, l := range c {
		if l == "" {
			out = append(out, "")
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(l, w, " "), "\n")...)
	}
	return out
}

var (
	borderFocus = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	borderPlain = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle  = lipgloss.NewStyle().Bold(true)
)

// box draws lines in a w by h bordered frame with the title in the top
// border and the scroll position in the bottom one. It returns exactly h
// lines of exactly w cells, and the offset it used after clamping.
func box(title string, lines []string, w, h, scroll int, focused bool) ([]string, int) {
	b := lipgloss.RoundedBorder()
	bs := borderPlain
	if focused {
		bs = borderFocus
	}
	inner, rows := w-2, h-2
	maxScroll := len(lines) - rows
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll = max(0, min(scroll, maxScroll))

	t := " " + ansi.Truncate(title, inner-3, "…") + " "
	top := bs.Render(b.TopLeft+b.Top) + titleStyle.Render(t)
	if d := inner - 1 - ansi.StringWidth(t); d > 0 {
		top += bs.Render(strings.Repeat(b.Top, d))
	}
	top += bs.Render(b.TopRight)

	foot := ""
	if maxScroll > 0 {
		foot = " " + strconv.Itoa(scroll+1) + "-" + strconv.Itoa(min(scroll+rows, len(lines))) + "/" + strconv.Itoa(len(lines)) + " "
	}
	foot = ansi.Truncate(foot, inner-1, "")
	bottom := bs.Render(b.BottomLeft) + bs.Render(strings.Repeat(b.Bottom, inner-ansi.StringWidth(foot)-1)) +
		foot + bs.Render(b.Bottom+b.BottomRight)

	out := make([]string, 0, h)
	out = append(out, top)
	for i := 0; i < rows; i++ {
		l := ""
		if scroll+i < len(lines) {
			l = lines[scroll+i]
		}
		out = append(out, bs.Render(b.Left)+fit(l, inner)+bs.Render(b.Right))
	}
	return append(out, bottom), scroll
}
