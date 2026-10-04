package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headStyle = lipgloss.NewStyle().Bold(true)
	selStyle  = lipgloss.NewStyle().Reverse(true)
	dimStyle  = lipgloss.NewStyle().Faint(true)
)

// cell pads s to w display cells, centred, through Lip Gloss.
func cell(s string, w int, a lipgloss.Position) string {
	return lipgloss.NewStyle().Width(w).Align(a).Render(s)
}

// truncate shortens s to w cells with an ellipsis.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func (m Model) taskLabel(i int) string {
	r := m.data().Rows[i]
	mark := "  "
	if r.Parent {
		mark = "▾ "
		if m.collapsed[r.ID] {
			mark = "▸ "
		}
	}
	s := strings.Repeat("  ", r.Depth) + mark + r.Title
	if r.Parent && m.collapsed[r.ID] {
		s += fmt.Sprintf(" (%d)", m.descendants(i))
	}
	return s
}

// foldLabel is the header of a folded run: an arrow and the leaf count.
func foldLabel(f Fold) string {
	if f.Side == "before" {
		return fmt.Sprintf("‹%d", f.Count)
	}
	return fmt.Sprintf("%d›", f.Count)
}

// View implements tea.Model: the data and the chrome.
func (m Model) View() string { return m.render(true) }

// Data returns the project data alone: the header, the rule and the visible
// rows, with no cursor, no padding and no status, column or help line.
func (m Model) Data() string { return m.render(false) }

// render draws the grid. The chrome is the selection cursor, the padding to
// the window height and the three lines below the table.
func (m Model) render(chrome bool) string {
	d := m.data()
	shown := m.shown()
	vis := m.visible()

	// Column widths come from every row, so scrolling never reflows them.
	idW := 2
	for _, r := range d.Rows {
		if w := lipgloss.Width(r.ID); w > idW {
			idW = w
		}
	}
	type col struct {
		head string
		w    int
		gate string
	}
	var cols []col
	var befores, afters []Fold
	for _, f := range d.Folded {
		if f.Side == "before" {
			befores = append(befores, f)
		} else {
			afters = append(afters, f)
		}
	}
	for _, f := range befores {
		cols = append(cols, col{head: foldLabel(f)})
	}
	for _, c := range shown {
		cols = append(cols, col{head: strip(c.Symbol, m.StripVS16), gate: c.Gate})
	}
	if n, t := m.hiddenCount(); n > 0 {
		cols = append(cols, col{head: fmt.Sprintf("⊘%d", t)})
	}
	for _, f := range afters {
		cols = append(cols, col{head: foldLabel(f)})
	}
	gateTotal := 0
	for i := range cols {
		w := lipgloss.Width(cols[i].head)
		if cols[i].gate != "" {
			for _, r := range d.Rows {
				for _, c := range r.Cells {
					if c.Gate == cols[i].gate {
						if cw := lipgloss.Width(strip(c.Symbols, m.StripVS16)); cw > w {
							w = cw
						}
					}
				}
			}
		}
		cols[i].w = w
		gateTotal += w + 1
	}
	taskW := 4
	for i := range d.Rows {
		if w := lipgloss.Width(m.taskLabel(i)); w > taskW {
			taskW = w
		}
	}
	if avail := m.width - idW - 1 - 1 - gateTotal; taskW > avail {
		taskW = avail
	}
	if taskW < 8 {
		taskW = 8
	}

	line := func(id, task string, cells []string) string {
		var b strings.Builder
		b.WriteString(cell(id, idW, lipgloss.Left))
		b.WriteString(" ")
		b.WriteString(cell(truncate(task, taskW), taskW, lipgloss.Left))
		for i, c := range cells {
			b.WriteString(" ")
			b.WriteString(cell(c, cols[i].w, lipgloss.Center))
		}
		return b.String()
	}

	var lines []string
	heads := make([]string, len(cols))
	rules := make([]string, len(cols))
	for i, c := range cols {
		heads[i] = c.head
		rules[i] = strings.Repeat("─", c.w)
	}
	lines = append(lines, headStyle.Render(line("Id", "Task", heads)))
	lines = append(lines, line(strings.Repeat("─", idW), strings.Repeat("─", taskW), rules))

	bh := m.bodyHeight()
	sel := m.selIndex(vis)
	first, end := m.top, m.top+bh
	if !chrome {
		first, end = 0, len(vis)
	}
	for p := first; p < end; p++ {
		if p >= len(vis) {
			lines = append(lines, line("", "", make([]string, len(cols))))
			continue
		}
		i := vis[p]
		r := d.Rows[i]
		cells := make([]string, len(cols))
		for ci, c := range cols {
			if c.gate == "" {
				continue
			}
			for _, rc := range r.Cells {
				if rc.Gate == c.gate {
					cells[ci] = strip(rc.Symbols, m.StripVS16)
				}
			}
		}
		s := line(r.ID, m.taskLabel(i), cells)
		if chrome && p == sel {
			s = selStyle.Render(s)
		}
		lines = append(lines, s)
	}

	if !chrome {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, dimStyle.Render(truncate(m.statusLine(vis), m.width)))
	lines = append(lines, truncate(m.columnLine(), m.width))
	lines = append(lines, dimStyle.Render(truncate("↑↓ move  ←→ collapse/expand  enter toggle  e/c all/glance  1-9 column  0 show all  w window  q quit", m.width)))
	return strings.Join(lines, "\n")
}

func (m Model) statusLine(vis []int) string {
	d := m.data()
	if m.msg != "" {
		return m.msg
	}
	last := m.top + m.bodyHeight()
	if last > len(vis) {
		last = len(vis)
	}
	s := fmt.Sprintf("%s window %d, rows %d-%d of %d, selected %s", d.Ref, d.Window, m.top+1, last, len(vis), m.selected)
	for _, f := range d.Folded {
		s += fmt.Sprintf(" | folded %s %s..%s %d", f.Side, f.From, f.To, f.Count)
	}
	return s
}

// columnLine lists the window's columns with the digit that toggles each;
// a hidden column shows in parentheses.
func (m Model) columnLine() string {
	var parts []string
	for i, c := range m.data().Columns {
		s := fmt.Sprintf("%d%s", i+1, strip(c.Symbol, m.StripVS16))
		if m.hidden[c.Gate] {
			s = "(" + s + ")"
		}
		parts = append(parts, s)
	}
	return "columns " + strings.Join(parts, " ")
}
