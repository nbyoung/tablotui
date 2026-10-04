package main

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the stand-in grid: an indented tree, an expanded set, a gate window
// and a set of hidden columns.
type Model struct {
	Views  Views
	Person Person
	Mode   Role // the role whose starting state shows; the key cycles the held roles
	Notice string

	Expanded map[string]bool
	First    string
	Last     string
	Hidden   map[string]bool

	Row, Col      int
	width, height int
	parent        map[string]string
}

// NewModel starts in the person's own role.
func NewModel(v Views, email string) Model {
	m := Model{Views: v, Person: Classify(v, email), Hidden: map[string]bool{}, parent: parents(v.Global)}
	m.apply(StartFor(v, m.Person, m.Person.Role()))
	return m
}

// apply takes a starting state. It leaves Hidden alone: the columns the
// viewer chose outlive a change of role.
func (m *Model) apply(s Start) {
	m.Mode, m.Expanded, m.First, m.Last = s.Mode, s.Expanded, s.First, s.Last
	m.Row, m.Col = 0, 0
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// VisibleRows lists the task rows whose ancestors are all expanded.
func (m Model) VisibleRows() []Row {
	var out []Row
	for _, r := range m.Views.Global.Rows {
		ok := true
		for a, has := m.parent[r.ID]; has; a, has = m.parent[a] {
			if !m.Expanded[a] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// VisibleColumns lists the gates in the window that are not hidden.
func (m Model) VisibleColumns() []string {
	var out []string
	in := false
	for _, g := range m.Views.Gates.Glance.Gates {
		if g.Key == m.First {
			in = true
		}
		if in && !m.Hidden[g.Key] {
			out = append(out, g.Key)
		}
		if g.Key == m.Last {
			break
		}
	}
	return out
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		rows, cols := m.VisibleRows(), m.VisibleColumns()
		m.Notice = ""
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.Row = min(m.Row+1, len(rows)-1)
		case "k", "up":
			m.Row = max(m.Row-1, 0)
		case "l", "right":
			m.Col = min(m.Col+1, len(cols)-1)
		case "h", "left":
			m.Col = max(m.Col-1, 0)
		case "enter", " ":
			if m.Row < len(rows) && rows[m.Row].Parent {
				e := cloneSet(m.Expanded)
				e[rows[m.Row].ID] = !e[rows[m.Row].ID]
				m.Expanded = e
			}
		case "x":
			if m.Col < len(cols) && len(cols) > 1 {
				h := cloneSet(m.Hidden)
				h[cols[m.Col]] = true
				m.Hidden = h
			}
		case "X":
			m.Hidden = map[string]bool{}
		case "E":
			e := map[string]bool{}
			for _, r := range m.Views.Global.Rows {
				if r.Parent {
					e[r.ID] = true
				}
			}
			m.Expanded = e
		case "C":
			m.Expanded = map[string]bool{}
		case "r":
			roles := m.Person.Roles()
			if len(roles) == 1 {
				m.Notice = fmt.Sprintf("one role only: %s", roles[0])
				break
			}
			next := roles[0]
			for i, r := range roles {
				if r == m.Mode {
					next = roles[(i+1)%len(roles)]
				}
			}
			m.apply(StartFor(m.Views, m.Person, next))
		}
		m.Row = max(min(m.Row, len(m.VisibleRows())-1), 0)
		m.Col = max(min(m.Col, len(m.VisibleColumns())-1), 0)
	}
	return m, nil
}

func cloneSet(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func keys(s map[string]bool) string {
	var out []string
	for k, v := range s {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ",")
}

// View implements tea.Model: the data with the chrome.
func (m Model) View() string { return m.Render(true) }

// Render draws the tree. The project data is the line naming the person and
// the role, the column headers and the task rows with their marks (* the
// status gate, > the next gate). The chrome is the status line (window,
// expanded, hidden, roles held, notices), the cursor, the column underline
// and the blank line.
func (m Model) Render(chrome bool) string {
	var b strings.Builder
	who := m.Person.Email
	if who == "" {
		who = "(nobody)"
	}
	title := fmt.Sprintf("tablotui f394  as %s  role: %s", who, m.Mode)
	if chrome {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}
	b.WriteString(title + "\n")
	if chrome {
		roles := make([]string, 0, 2)
		for _, r := range m.Person.Roles() {
			roles = append(roles, string(r))
		}
		fmt.Fprintf(&b, "window %s..%s  expanded %s  hidden %s  roles %s\n", m.First, m.Last, keys(m.Expanded), keys(m.Hidden), strings.Join(roles, ","))
		if m.Notice != "" {
			b.WriteString(m.Notice + "\n")
		} else {
			b.WriteString("\n")
		}
	}
	cols := m.VisibleColumns()
	lead, w := "", 33
	if chrome {
		lead, w = "  ", 32
	}
	fmt.Fprintf(&b, "%s%-*s", lead, w, "task")
	for i, c := range cols {
		cell := fmt.Sprintf(" %-5.5s", c)
		if chrome && i == m.Col {
			cell = lipgloss.NewStyle().Underline(true).Render(cell)
		}
		b.WriteString(cell)
	}
	b.WriteString("\n")
	for i, r := range m.VisibleRows() {
		mark := " "
		if r.Parent {
			mark = "+"
			if m.Expanded[r.ID] {
				mark = "-"
			}
		}
		cur := ""
		if chrome {
			cur = " "
			if i == m.Row {
				cur = ">"
			}
		}
		label := fmt.Sprintf("%s%s%s %s %s", cur, strings.Repeat("  ", r.Depth), mark, r.ID, r.Title)
		fmt.Fprintf(&b, "%-*s", w+len(lead), label)
		for _, c := range cols {
			cell := "."
			switch {
			case r.Status.Gate == c && r.NextGate == c:
				cell = "*>"
			case r.Status.Gate == c:
				cell = "*"
			case r.NextGate == c:
				cell = ">"
			}
			fmt.Fprintf(&b, " %-5s", cell)
		}
		b.WriteString("\n")
	}
	if chrome {
		return b.String()
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n") + "\n"
}
