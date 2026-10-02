// Command 518e is the functional prototype of tablotui's actions.
//
// A key on the selected task opens an action: record a status, review a gate,
// reaffirm, authorise. The model composes the commit with the write commands
// in write.go (a verbatim copy of tabloio's b618), shows the exact message and
// file diff, and applies it only after confirmation. Both git steps run in a
// tea.Cmd, never inside Update. The list of tasks is the smallest stand-in for
// the grid and the panes that other tasks build.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// task is one row of the stand-in list.
type task struct{ ID, Title, Gate, State, Reason string }

type mode int

const (
	modeList     mode = iota // browsing the tasks
	modeForm                 // choosing the fields of an action
	modeComposed             // waiting for the commit to be composed and shown
	modePreview              // showing the commit, waiting for y or n
	modeApplying             // git runs in a command
	modeError                // showing a failure
)

// Messages that commands return to Update.
type (
	composedMsg struct {
		commit Commit
		shown  string
		err    error
	}
	appliedMsg struct {
		subject string
		err     error // the apply error
		rollErr error // a failure to restore the repository after err
	}
)

// model is the whole interface state.
type model struct {
	repo   string
	env    []string // added to git's environment; nil takes the caller's identity
	agent  Agent
	gates  []string
	states []string
	reason []string // first entry is empty: no reason
	tasks  []task
	cursor int

	mode   mode
	action string // "record", "review", "reaffirm" or "authorise"
	form   formState
	shown  string // the commit as Show prints it
	commit Commit
	status string // the last result, one line
	err    error

	width, height int
}

// formState holds the input of the record and review actions.
type formState struct {
	fields []string // "gate", "state", "reason", "note", "self"
	focus  int
	gate   int
	state  int
	reason int
	self   bool
	note   textinput.Model
}

func newModel(repo string, env []string, a Agent) (model, error) {
	m := model{repo: repo, env: env, agent: a, width: 80, height: 24}
	if err := m.load(); err != nil {
		return m, err
	}
	return m, nil
}

// load reads the stand-in data: the gates, states and reasons from gates.yaml
// and the tasks with their status. The real interface takes both from tablo.
func (m *model) load() error {
	b, err := os.ReadFile(filepath.Join(m.repo, ".tableaux", "gates.yaml"))
	if err != nil {
		return err
	}
	keyRe := regexp.MustCompile(`\{\s*key:\s*([a-z_]+)`)
	m.gates, m.states, m.reason = nil, nil, []string{""}
	section := ""
	for _, l := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(l, "gates:"), strings.HasPrefix(l, "states:"), strings.HasPrefix(l, "reasons:"):
			section = strings.TrimSuffix(l, ":")
		case keyRe.MatchString(l):
			k := keyRe.FindStringSubmatch(l)[1]
			switch section {
			case "gates":
				m.gates = append(m.gates, k)
			case "states":
				m.states = append(m.states, k)
			case "reasons":
				m.reason = append(m.reason, k)
			}
		}
	}
	files, err := filepath.Glob(filepath.Join(m.repo, ".tableaux", "tasks", "*.yaml"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	m.tasks = nil
	field := func(text, key string) string {
		re := regexp.MustCompile(`(?m)^` + key + `:\s*(.*?)\s*$`)
		if s := re.FindStringSubmatch(text); s != nil {
			return strings.Trim(s[1], `"`)
		}
		return ""
	}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".yaml")
		tb, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		t := task{ID: id, Title: field(string(tb), "title"), Gate: "undefined", State: "undefined"}
		if sb, err := os.ReadFile(filepath.Join(m.repo, statusPath(id))); err == nil {
			t.Gate, t.State, t.Reason = field(string(sb), "gate"), field(string(sb), "state"), field(string(sb), "reason")
		}
		m.tasks = append(m.tasks, t)
	}
	if m.cursor >= len(m.tasks) {
		m.cursor = 0
	}
	return nil
}

func (m model) Init() tea.Cmd { return nil }

func indexIn(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return 0
}

// openForm prepares the input for record or review from the selected task.
func (m model) openForm(action string) model {
	t := m.tasks[m.cursor]
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "note (optional)"
	ti.CharLimit = 200
	ti.Width = 40
	ti.Cursor.SetMode(cursor.CursorStatic) // no blink command to run
	f := formState{
		gate: indexIn(m.gates, t.Gate), state: indexIn(m.states, t.State),
		reason: indexIn(m.reason, t.Reason), note: ti,
	}
	if action == "record" {
		f.fields = []string{"gate", "state", "reason", "note", "self"}
	} else {
		f.fields = []string{"gate"}
	}
	m.form, m.action, m.mode = f, action, modeForm
	return m
}

// composeCmd composes the commit and renders it. Show runs git diff, so this
// is a command as well.
func composeCmd(m model) tea.Cmd {
	repo, a, id, action, f := m.repo, m.agent, m.tasks[m.cursor].ID, m.action, m.form
	gate := m.gates[f.gate]
	status := Status{ID: id, Gate: gate, State: m.states[f.state], Reason: m.reason[f.reason],
		Note: strings.TrimSpace(f.note.Value()), SelfReview: f.self}
	return func() tea.Msg {
		var c Commit
		var err error
		switch action {
		case "record":
			c, err = Record(repo, status, a)
		case "review":
			c, err = Review(repo, []string{id}, gate, a)
		case "reaffirm":
			c, err = Reaffirm(repo, []string{id}, a)
		case "authorise":
			c, err = Authorise(repo, []string{id}, a)
		default:
			err = fmt.Errorf("unknown action %q", action)
		}
		if err != nil {
			return composedMsg{err: err}
		}
		shown, err := c.Show(repo)
		return composedMsg{commit: c, shown: shown, err: err}
	}
}

// applyCmd runs Apply, which writes files and runs git commit. When Apply
// fails it has already written and staged the files, so the command puts the
// repository back as it was: Apply itself leaves it dirty.
func applyCmd(repo string, env []string, c Commit) tea.Cmd {
	return func() tea.Msg {
		type snap struct {
			data   []byte
			exists bool
		}
		before := map[string]snap{}
		for p := range c.Files {
			b, err := os.ReadFile(filepath.Join(repo, p))
			before[p] = snap{b, err == nil}
		}
		err := c.Apply(repo, env)
		if err == nil {
			return appliedMsg{subject: c.Subject}
		}
		var rollErr error
		for p, s := range before {
			full := filepath.Join(repo, p)
			if s.exists {
				rollErr = errors.Join(rollErr, os.WriteFile(full, s.data, 0o644))
			} else {
				rollErr = errors.Join(rollErr, os.Remove(full))
			}
			rollErr = errors.Join(rollErr, git(repo, env, "reset", "-q", "--", p))
		}
		return appliedMsg{subject: c.Subject, err: err, rollErr: rollErr}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case composedMsg:
		if msg.err != nil {
			m.mode, m.err = modeError, msg.err
			return m, nil
		}
		m.mode, m.commit, m.shown = modePreview, msg.commit, msg.shown
	case appliedMsg:
		if msg.err != nil {
			m.mode, m.err = modeError, msg.err
			if msg.rollErr != nil {
				m.err = fmt.Errorf("%w; restoring the repository also failed: %v", msg.err, msg.rollErr)
			}
			return m, nil
		}
		m.mode, m.status = modeList, "committed: "+msg.subject
		if err := m.load(); err != nil {
			m.mode, m.err = modeError, err
		}
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	switch m.mode {
	case modeList:
		switch s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "r":
			return m.openForm("record"), nil
		case "v":
			return m.openForm("review"), nil
		case "a", "u":
			m.action, m.mode, m.status = map[string]string{"a": "reaffirm", "u": "authorise"}[s], modeComposed, ""
			return m, composeCmd(m)
		}
	case modeForm:
		return m.formKey(k)
	case modePreview:
		switch s {
		case "y", "enter":
			m.mode = modeApplying
			return m, applyCmd(m.repo, m.env, m.commit)
		case "n", "esc":
			m.mode, m.status = modeList, "cancelled: nothing changed"
		}
	case modeError:
		if s == "enter" || s == "esc" || s == "q" {
			m.mode, m.err = modeList, nil
		}
	case modeComposed, modeApplying:
		if s == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) formKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.form
	name := f.fields[f.focus]
	switch k.String() {
	case "esc":
		m.mode, m.status = modeList, "cancelled: nothing changed"
		return m, nil
	case "enter":
		m.mode, m.status = modeComposed, ""
		return m, composeCmd(m)
	case "tab", "down":
		f.focus = (f.focus + 1) % len(f.fields)
	case "shift+tab", "up":
		f.focus = (f.focus + len(f.fields) - 1) % len(f.fields)
	case "left", "right":
		d := 1
		if k.String() == "left" {
			d = -1
		}
		switch name {
		case "gate":
			f.gate = (f.gate + d + len(m.gates)) % len(m.gates)
		case "state":
			f.state = (f.state + d + len(m.states)) % len(m.states)
		case "reason":
			f.reason = (f.reason + d + len(m.reason)) % len(m.reason)
		case "self":
			f.self = !f.self
		case "note":
			f.note, _ = f.note.Update(k)
		}
	default:
		if name == "note" {
			f.note, _ = f.note.Update(k)
		}
	}
	if f.fields[f.focus] == "note" {
		_ = f.note.Focus()
	} else {
		f.note.Blur()
	}
	m.form = f
	return m, nil
}

var (
	dim  = lipgloss.NewStyle().Faint(true)
	sel  = lipgloss.NewStyle().Bold(true)
	add  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	del  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	fail = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

func (m model) View() string {
	var b strings.Builder
	for i, t := range m.tasks {
		line := fmt.Sprintf("%s %s  %-24s %s/%s", map[bool]string{true: ">", false: " "}[i == m.cursor],
			t.ID, t.Title, t.Gate, t.State)
		if t.Reason != "" {
			line += " (" + t.Reason + ")"
		}
		if i == m.cursor {
			line = sel.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	switch m.mode {
	case modeList:
		if m.status != "" {
			b.WriteString(m.status + "\n")
		}
		b.WriteString(dim.Render("r record  v review  a reaffirm  u authorise  q quit") + "\n")
	case modeForm:
		b.WriteString(m.formView())
	case modeComposed:
		b.WriteString("composing the commit...\n")
	case modePreview:
		for _, l := range strings.Split(strings.TrimRight(m.shown, "\n"), "\n") {
			switch {
			case strings.HasPrefix(l, "+"):
				l = add.Render(l)
			case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
				l = del.Render(l)
			}
			b.WriteString(l + "\n")
		}
		b.WriteString("\n" + sel.Render("Apply this commit? y/Enter apply, n/Esc cancel") + "\n")
	case modeApplying:
		b.WriteString("committing...\n")
	case modeError:
		b.WriteString(fail.Render("failed: ") + strings.TrimSpace(m.err.Error()) + "\n")
		b.WriteString(dim.Render("The repository is as it was. Enter returns to the list.") + "\n")
	}
	return b.String()
}

func (m model) formView() string {
	f := m.form
	var b strings.Builder
	t := m.tasks[m.cursor]
	fmt.Fprintf(&b, "%s task %s\n", m.action, t.ID)
	for i, name := range f.fields {
		var v string
		switch name {
		case "gate":
			v = "< " + m.gates[f.gate] + " >"
		case "state":
			v = "< " + m.states[f.state] + " >"
		case "reason":
			r := m.reason[f.reason]
			if r == "" {
				r = "none"
			}
			v = "< " + r + " >"
		case "note":
			v = f.note.View()
		case "self":
			v = "< " + map[bool]string{true: "yes", false: "no"}[f.self] + " >"
		}
		label := map[string]string{"gate": "gate", "state": "state", "reason": "reason", "note": "note", "self": "self-review"}[name]
		mark := " "
		if i == f.focus {
			mark = ">"
		}
		fmt.Fprintf(&b, "%s %-12s %s\n", mark, label, v)
	}
	b.WriteString(dim.Render("Tab next field, Left/Right choose, Enter show the commit, Esc cancel") + "\n")
	return b.String()
}

func main() {
	repo := flag.String("repo", ".", "repository with a .tableaux directory")
	model_ := flag.String("model", "", "Model trailer (empty: a person's commit, no trailer)")
	name := flag.String("name", "", "Co-Authored-By display name")
	flag.Parse()
	m, err := newModel(*repo, nil, Agent{Model: *model_, Name: *name})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
