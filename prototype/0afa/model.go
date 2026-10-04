package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The four panes, in the order of the focus cycle after the list.
const (
	paneDefinition = iota
	paneBlockage
	paneQueue
	paneHistory
	numPanes
)

// pane holds one pane's content and its own scroll offset.
type pane struct {
	title   string
	content Content
	scroll  int
}

// Model is the Bubble Tea model: the stand-in list and the four panes.
type Model struct {
	src     Source
	person  string
	persons []string
	ref     string
	rows    []row
	sel     int
	listTop int
	panes   [numPanes]pane

	focus  int // 0 the list, 1..numPanes a pane
	active int // the pane shown alone when space is short, or zoomed
	zoom   bool

	w, h     int
	stripVS1 bool // drop U+FE0F so every terminal draws one-cell text glyphs
}

// NewModel loads the task list and the panes for the first task.
func NewModel(src Source, person string, persons []string, stripVS16 bool) (Model, error) {
	b, err := src.Load(ViewGlobal, Params{})
	if err != nil {
		return Model{}, err
	}
	rows, err := globalRows(b)
	if err != nil {
		return Model{}, err
	}
	m := Model{src: src, person: person, persons: persons, rows: rows, w: 100, h: 30, stripVS1: stripVS16}
	m.refresh(true)
	return m, nil
}

func (m Model) selected() string {
	if len(m.rows) == 0 {
		return ""
	}
	return m.rows[m.sel].ID
}

// refresh loads each pane's view for the selection. The queue depends on the
// person alone, so a change of selection only marks its items.
func (m *Model) refresh(resetScroll bool) {
	task := m.selected()
	p := Params{Task: task, Person: m.person, Ref: m.ref}
	load := func(i int, name string, v View, conv func([]byte) (Content, error)) {
		m.panes[i].title = name
		b, err := m.src.Load(v, p)
		var c Content
		if err == nil {
			c, err = conv(b)
		}
		if err != nil {
			c = noData(v, p, err)
		}
		if m.stripVS1 {
			for k := range c {
				c[k] = strings.ReplaceAll(c[k], "️", "")
			}
		}
		m.panes[i].content = c
		if resetScroll {
			m.panes[i].scroll = 0
		}
	}
	load(paneDefinition, "Task "+task, ViewDefinition, definitionContent)
	load(paneBlockage, "Blockage "+task, ViewBlockage, func(b []byte) (Content, error) { return blockageContent(b, task) })
	load(paneQueue, "Queue "+m.person, ViewQueue, func(b []byte) (Content, error) { return queueContent(b, m.person, task) })
	load(paneHistory, "History "+task, ViewHistory, historyContent)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.setFocus((m.focus + 1) % (numPanes + 1))
		case "shift+tab":
			m.setFocus((m.focus + numPanes) % (numPanes + 1))
		case "esc":
			m.setFocus(0)
		case "1", "2", "3", "4":
			m.setFocus(int(msg.String()[0] - '0'))
		case "z":
			m.zoom = !m.zoom
		case "p":
			for i, p := range m.persons {
				if p == m.person {
					m.person = m.persons[(i+1)%len(m.persons)]
					break
				}
			}
			m.refresh(false)
			m.panes[paneQueue].scroll = 0
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "pgup":
			m.move(-m.pageSize())
		case "pgdown":
			m.move(m.pageSize())
		case "home", "g":
			m.move(-1 << 20)
		case "end", "G":
			m.move(1 << 20)
		}
	}
	m.clamp()
	return m, nil
}

func (m *Model) setFocus(f int) {
	m.focus = f
	if f > 0 {
		m.active = f - 1
	}
}

// move moves the selection when the list has focus, and scrolls the focused
// pane otherwise.
func (m *Model) move(d int) {
	if m.focus == 0 {
		old := m.sel
		m.sel = max(0, min(len(m.rows)-1, m.sel+d))
		if m.sel != old {
			m.refresh(true)
		}
		return
	}
	m.panes[m.focus-1].scroll += d
}

func (m Model) pageSize() int {
	return max(1, m.h-4)
}

// clamp keeps every scroll offset inside its pane and the list cursor in view.
func (m *Model) clamp() {
	l := m.layout()
	for _, b := range l.boxes {
		if b.pane < 0 {
			rows := b.h - 2
			if m.sel < m.listTop {
				m.listTop = m.sel
			}
			if m.sel >= m.listTop+rows {
				m.listTop = m.sel - rows + 1
			}
			m.listTop = max(0, m.listTop)
			continue
		}
		n := len(wrap(m.panes[b.pane].content, b.w-2)) - (b.h - 2)
		m.panes[b.pane].scroll = max(0, min(m.panes[b.pane].scroll, n))
	}
}

// boxRect places one box: pane -1 is the list.
type boxRect struct{ pane, x, y, w, h int }

type layout struct {
	tooSmall bool
	listW    int
	boxes    []boxRect
	mode     string
}

const (
	minW, minH = 40, 8
	paneMinH   = 6
)

// layout decides which boxes show and where. Wide and tall: the list, then
// the panes in two columns of two. Narrower or shorter: the panes in one
// stack. Too short for a stack, or zoomed: the active pane alone.
func (m Model) layout() layout {
	bodyH := m.h - 1 // the last line is the key help
	if m.w < minW || m.h < minH {
		return layout{tooSmall: true}
	}
	listW := max(20, min(34, m.w/4))
	pw := m.w - listW
	l := layout{listW: listW, boxes: []boxRect{{-1, 0, 0, listW, bodyH}}}
	grid := pw >= 70 && bodyH >= 2*paneMinH
	stack := bodyH >= numPanes*paneMinH
	switch {
	case m.zoom || !grid && !stack:
		l.mode = "single"
		l.boxes = append(l.boxes, boxRect{m.active, listW, 0, pw, bodyH})
	case grid:
		l.mode = "grid"
		lw := pw / 2
		th := bodyH / 2
		l.boxes = append(l.boxes,
			boxRect{paneDefinition, listW, 0, lw, th},
			boxRect{paneBlockage, listW + lw, 0, pw - lw, th},
			boxRect{paneQueue, listW, th, lw, bodyH - th},
			boxRect{paneHistory, listW + lw, th, pw - lw, bodyH - th})
	default:
		l.mode = "stack"
		y := 0
		for i := 0; i < numPanes; i++ {
			hh := bodyH / numPanes
			if i < bodyH%numPanes {
				hh++
			}
			l.boxes = append(l.boxes, boxRect{i, listW, y, pw, hh})
			y += hh
		}
	}
	return l
}

var (
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	helpStyle   = lipgloss.NewStyle().Faint(true)
)

const help = "tab focus  j/k move or scroll  1-4 pane  z zoom  p person  q quit"

// View implements tea.Model. It returns exactly m.h lines of exactly m.w cells.
func (m Model) View() string {
	blank := func(s string) string { return fit(s, m.w) }
	l := m.layout()
	if l.tooSmall {
		out := make([]string, m.h)
		for i := range out {
			out[i] = blank("")
		}
		if m.h > 0 {
			out[0] = blank("Terminal too small")
		}
		return strings.Join(out, "\n")
	}
	bodyH := m.h - 1
	canvas := make([]string, bodyH)
	for i := range canvas {
		canvas[i] = strings.Repeat(" ", m.w)
	}
	for _, b := range l.boxes {
		var lines []string
		var title string
		scroll := 0
		focused := false
		if b.pane < 0 {
			title, lines, scroll, focused = "Tasks", m.listLines(b.w-2), m.listTop, m.focus == 0
		} else {
			p := m.panes[b.pane]
			title, lines, scroll, focused = p.title, wrap(p.content, b.w-2), p.scroll, m.focus == b.pane+1
		}
		bx, _ := box(title, lines, b.w, b.h, scroll, focused)
		for i, s := range bx {
			row := canvas[b.y+i]
			canvas[b.y+i] = ansi.Cut(row, 0, b.x) + s + ansi.Cut(row, b.x+b.w, m.w)
		}
	}
	canvas = append(canvas, helpStyle.Render(fit(help, m.w)))
	return strings.Join(canvas, "\n")
}

func (m Model) listLines(w int) []string {
	out := make([]string, len(m.rows))
	for i, r := range m.rows {
		s := strings.Repeat("  ", r.Depth) + r.ID + " " + r.Title
		s = fit(" "+s, w)
		if i == m.sel {
			s = cursorStyle.Render(s)
		}
		out[i] = s
	}
	return out
}

// Dump prints the project data alone: each visible pane's title line naming
// its view and task or person, then its whole content wrapped to the window
// width, in layout order. It leaves out the list (a stand-in), borders, focus
// marks, scroll offsets, clipping to the pane height, key help and padding.
func (m Model) Dump() string {
	l := m.layout()
	if l.tooSmall {
		return "Terminal too small"
	}
	var out []string
	for _, b := range l.boxes {
		if b.pane < 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		p := m.panes[b.pane]
		out = append(out, ansi.Truncate(p.title, m.w, ""))
		out = append(out, wrap(p.content, m.w)...)
	}
	return strings.Join(out, "\n")
}
