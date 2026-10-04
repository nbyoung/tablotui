package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the tableau grid: a Bubble Tea model over global tableau view data.
type Model struct {
	variants  []Data // the window variants tablo emitted; w cycles them
	cur       int
	width     int
	height    int
	collapsed map[string]bool
	hidden    map[string]bool // gate names hidden by the viewer
	selected  string          // id of the selected row
	top       int             // index of the first visible row drawn
	path      string          // settings file; empty keeps nothing
	msg       string
	// StripVS16 removes U+FE0F from every symbol, so a terminal that keeps
	// the narrow base one cell wide and one that widens it agree.
	StripVS16 bool
}

// chromeLines counts the lines the window spends outside the body: the header
// and its rule above, the status, column and help lines below.
const chromeLines = 5

// NewModel builds the model over one or more window variants, reading the
// hidden gates from the settings path (empty: keep none).
func NewModel(variants []Data, settingsPath string) Model {
	m := Model{
		variants:  variants,
		width:     80,
		height:    24,
		collapsed: map[string]bool{},
		path:      settingsPath,
	}
	hidden, err := LoadHidden(settingsPath)
	if err != nil {
		m.msg = "settings: " + err.Error()
	}
	m.hidden = hidden
	if v := m.visible(); len(v) > 0 {
		m.selected = m.data().Rows[v[0]].ID
	}
	return m
}

func (m Model) data() Data { return m.variants[m.cur] }

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// visible lists the indexes of the rows whose ancestors are all expanded.
func (m Model) visible() []int {
	rows := m.data().Rows
	var out []int
	skip := -1 // depth of the collapsed ancestor being skipped, or -1
	for i, r := range rows {
		if skip >= 0 && r.Depth > skip {
			continue
		}
		skip = -1
		out = append(out, i)
		if r.Parent && m.collapsed[r.ID] {
			skip = r.Depth
		}
	}
	return out
}

// descendants counts the rows below row i.
func (m Model) descendants(i int) int {
	rows := m.data().Rows
	n := 0
	for j := i + 1; j < len(rows) && rows[j].Depth > rows[i].Depth; j++ {
		n++
	}
	return n
}

// selIndex returns the position of the selection in the visible list.
func (m Model) selIndex(vis []int) int {
	rows := m.data().Rows
	for p, i := range vis {
		if rows[i].ID == m.selected {
			return p
		}
	}
	return 0
}

func (m Model) bodyHeight() int {
	if h := m.height - chromeLines; h > 1 {
		return h
	}
	return 1
}

// settle keeps the selection visible and the scroll offset in range.
func (m *Model) settle() {
	vis := m.visible()
	if len(vis) == 0 {
		return
	}
	rows := m.data().Rows
	p := m.selIndex(vis)
	m.selected = rows[vis[p]].ID
	bh := m.bodyHeight()
	if p < m.top {
		m.top = p
	}
	if p >= m.top+bh {
		m.top = p - bh + 1
	}
	if max := len(vis) - bh; m.top > max {
		m.top = max
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m *Model) move(vis []int, p int) {
	if p < 0 {
		p = 0
	}
	if p >= len(vis) {
		p = len(vis) - 1
	}
	m.selected = m.data().Rows[vis[p]].ID
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.msg = ""
		m.key(msg.String())
	}
	m.settle()
	return m, nil
}

func (m *Model) key(k string) {
	vis := m.visible()
	if len(vis) == 0 {
		return
	}
	rows := m.data().Rows
	p := m.selIndex(vis)
	r := rows[vis[p]]
	switch k {
	case "up", "k":
		m.move(vis, p-1)
	case "down", "j":
		m.move(vis, p+1)
	case "pgup":
		m.move(vis, p-m.bodyHeight())
	case "pgdown":
		m.move(vis, p+m.bodyHeight())
	case "home", "g":
		m.move(vis, 0)
	case "end", "G":
		m.move(vis, len(vis)-1)
	case "enter", " ":
		if r.Parent {
			m.collapsed[r.ID] = !m.collapsed[r.ID]
		}
	case "right", "l":
		if r.Parent && m.collapsed[r.ID] {
			m.collapsed[r.ID] = false
		} else if r.Parent {
			m.move(vis, p+1)
		}
	case "left", "h":
		if r.Parent && !m.collapsed[r.ID] {
			m.collapsed[r.ID] = true
		} else {
			for q := p - 1; q >= 0; q-- {
				if rows[vis[q]].Depth < r.Depth {
					m.move(vis, q)
					break
				}
			}
		}
	case "e": // expand all
		m.collapsed = map[string]bool{}
	case "c": // collapse to depth one, the glance level
		for _, row := range rows {
			if row.Parent && row.Depth >= 1 {
				m.collapsed[row.ID] = true
			}
		}
	case "w":
		m.cur = (m.cur + 1) % len(m.variants)
	case "0":
		m.hidden = map[string]bool{}
		m.save()
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			cols := m.data().Columns
			if i := int(k[0] - '1'); i < len(cols) {
				g := cols[i].Gate
				m.hidden[g] = !m.hidden[g]
				m.save()
			}
		}
	}
}

func (m *Model) save() {
	if err := SaveHidden(m.path, m.hidden); err != nil {
		m.msg = "settings: " + err.Error()
	}
}

// shown returns the columns of the window the viewer has not hidden.
func (m Model) shown() []Column {
	var out []Column
	for _, c := range m.data().Columns {
		if !m.hidden[c.Gate] {
			out = append(out, c)
		}
	}
	return out
}

// hiddenCount counts the leaf tasks whose current gate lies in a hidden column.
func (m Model) hiddenCount() (cols, tasks int) {
	d := m.data()
	set := map[string]bool{}
	for _, c := range d.Columns {
		if m.hidden[c.Gate] {
			cols++
			set[c.Gate] = true
		}
	}
	for _, r := range d.Rows {
		if !r.Parent && set[r.Status.Gate] {
			tasks++
		}
	}
	return cols, tasks
}

func strip(s string, on bool) string {
	if on {
		return strings.ReplaceAll(s, "️", "")
	}
	return s
}
