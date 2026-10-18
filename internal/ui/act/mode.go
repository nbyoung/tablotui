package act

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui"
)

// stage is where a mode stands.
type stage int

const (
	stageForm     stage = iota // record: the viewer sets the fields
	stagePlanning              // record: Record runs
	stageConfirm               // the plan shows, and y commits
	stageApplying              // Apply runs
	stageStopped               // a plan or a commit stopped, and the screen says why
)

// mode is the one ui.Mode of the actions: the record form, the
// confirmation of a plan, or the screen that says why one stopped.
type mode struct {
	a           *Actions
	kind        kind
	id, gate    string
	name        string // "<command> <id>", with the gate for a review
	uncommitted bool   // Uncommitted listed a path
	stage       stage

	form     *form
	messages []string // the form: why the last plan stopped, its first line "refused:" or "no plan:"

	plan *action.Plan

	head  string   // stopped: the head
	lines []string // stopped: the lines that state the error
	top   int      // the first line the pager shows
}

// The lines the mode adds.
const (
	uncommittedLine = "uncommitted edits under .tableaux; the plan reads HEAD"
	promptLine      = "Commit? y commits, n cancels"
)

// help bindings: the help line of each stage. Each holds keys only so that
// the frame counts it enabled; the mode matches keys by name.
var (
	helpPlan   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "plan"))
	helpField  = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "field"))
	helpChoice = key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←→", "choice"))
	helpCancel = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
	helpCommit = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "commit"))
	helpNo     = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "cancel"))
	helpScroll = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "scroll"))
	helpClose  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "close"))
)

// confirm shows a plan.
func (m *mode) confirm(p *action.Plan) {
	m.plan, m.stage, m.top, m.messages = p, stageConfirm, 0, nil
}

// stopPlan shows why a plan of review, reaffirm or authorise stopped.
func (m *mode) stopPlan(err error) {
	m.head = m.name + ": no plan"
	if refused(err) {
		m.head = m.name + ": refused"
	}
	m.lines, m.stage, m.top = errLines(err), stageStopped, 0
}

// stopApply shows why a commit stopped, or what a hook did to it.
func (m *mode) stopApply(res action.Result, err error) {
	m.lines, m.stage, m.top = errLines(err), stageStopped, 0
	switch {
	case res.Commit != "":
		m.head = m.name + ": committed " + short(res.Commit) + ", and a hook altered the commit"
	case refused(err):
		m.head = m.name + ": refused"
		m.lines = append(m.lines, "", "nothing is written; close this and press the key again")
	default:
		m.head = m.name + ": not committed"
		m.lines = append(m.lines, "", "the checkout stands as it stood before the commit")
	}
}

// Update implements ui.Mode.
func (m *mode) Update(msg tea.Msg, c ui.Context) (ui.Mode, tea.Cmd) {
	switch msg := msg.(type) {
	case plannedMsg:
		if msg.to != m || m.stage != stagePlanning {
			return m, nil
		}
		if msg.err != nil {
			head := "no plan:"
			if refused(msg.err) {
				head = "refused:"
			}
			m.stage, m.messages = stageForm, append([]string{head}, errLines(msg.err)...)
			return m, nil
		}
		m.confirm(msg.plan)
	case appliedMsg:
		if msg.to != m || m.stage != stageApplying {
			return m, nil
		}
		if msg.err == nil {
			return nil, nil
		}
		m.stopApply(msg.res, msg.err)
	case tea.PasteMsg:
		if m.stage == stageForm {
			m.form.insert(msg.Content)
		}
	case tea.KeyPressMsg:
		return m.key(msg, c)
	}
	return m, nil
}

// key handles a key in the stage the mode stands at.
func (m *mode) key(k tea.KeyPressMsg, c ui.Context) (ui.Mode, tea.Cmd) {
	s := k.String()
	switch m.stage {
	case stageForm:
		if m.form.key(k) {
			m.stage = stagePlanning
			return m, m.a.plan(m, m.form.recording())
		}
	case stageConfirm:
		switch s {
		case "y":
			m.stage = stageApplying
			return m, m.a.apply(m, m.plan)
		case "n", "q":
			return nil, notice("not committed", false)
		}
		m.scroll(s, c)
	case stageStopped:
		switch s {
		case "enter", "n", "q":
			return nil, nil
		}
		m.scroll(s, c)
	}
	return m, nil
}

// scroll moves the pager by a key and clamps it to the lines.
func (m *mode) scroll(s string, c ui.Context) {
	n, h := len(m.pagerLines(c.Width, c)), m.pagerHeight(c.Width, c.Height, c)
	switch s {
	case "up", "k":
		m.top--
	case "down", "j":
		m.top++
	case "pgup":
		m.top -= max(1, h)
	case "pgdown", "space":
		m.top += max(1, h)
	case "home", "g":
		m.top = 0
	case "end", "G":
		m.top = n
	}
	m.top = max(0, min(m.top, n-h))
}

// pagerLines returns the lines the pager holds, wrapped to w: the plan, or
// the lines of the error.
func (m *mode) pagerLines(w int, c ui.Context) []string {
	src := m.lines
	if m.stage == stageConfirm || m.stage == stageApplying {
		text := strings.TrimSuffix(m.plan.String(), "\n")
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\t", "    "), "\r", "")
		src = strings.Split(text, "\n")
	}
	return wrap(src, w, c.Measure)
}

// wrap wraps each line to w cells, at spaces where it can and by cells where
// it cannot.
func wrap(lines []string, w int, m ui.Measure) []string {
	var out []string
	for _, l := range lines {
		if l == "" || w <= 0 {
			out = append(out, l)
			continue
		}
		out = append(out, strings.Split(m.Method.Wrap(m.Clean(l), w, ""), "\n")...)
	}
	return out
}

// styled is one line of a footer and its style.
type styled struct {
	text  string
	style lipgloss.Style
}

// footer returns the lines beneath the plan, the prompt last.
func (m *mode) footer(c ui.Context) []styled {
	var out []styled
	switch ws := m.plan.Warnings; len(ws) {
	case 0:
	case 1:
		out = append(out, styled{"warning: " + ws[0].Code + ": " + ws[0].Text, c.Styles.Acts})
	default:
		codes := make([]string, len(ws))
		for i, w := range ws {
			codes[i] = w.Code
		}
		out = append(out, styled{fmt.Sprintf("%d warnings: %s; the end of the plan states them", len(ws), strings.Join(codes, ", ")), c.Styles.Acts})
	}
	if m.uncommitted {
		out = append(out, styled{uncommittedLine, c.Styles.Muted})
	}
	if v := m.a.o.Viewer; v != "" && v != m.plan.Author.Email && v != m.plan.Committer.Email {
		out = append(out, styled{"drawn for " + v + "; the commit is " + m.plan.Author.Email + "'s", c.Styles.Muted})
	}
	return append(out, styled{promptLine, c.Styles.Header})
}

// pagerHeight returns the lines the pager takes of a body h lines high.
func (m *mode) pagerHeight(w, h int, c ui.Context) int {
	if m.stage == stageStopped {
		return max(0, h-2)
	}
	ph, _ := m.confirmLayout(h, c)
	return ph
}

// confirmLayout returns the pager's height at the confirmation, and whether
// a rule stands above the footer.
func (m *mode) confirmLayout(h int, c ui.Context) (int, bool) {
	ph := max(0, h-len(m.footer(c)))
	if ph >= 4 {
		return ph - 1, true
	}
	return ph, false
}

// View implements ui.Mode.
func (m *mode) View(w, h int, c ui.Context) string {
	var out []string
	switch m.stage {
	case stageForm, stagePlanning:
		out = m.formView(w, h, c)
	case stageConfirm, stageApplying:
		ph, rule := m.confirmLayout(h, c)
		out = m.pagerView(w, ph, c, true)
		if rule {
			out = append(out, strings.Repeat("─", w))
		}
		foot := m.footer(c)
		foot = foot[max(0, len(foot)-h):]
		for _, f := range foot {
			out = append(out, f.style.Render(c.Measure.Fit(f.text, w)))
		}
	case stageStopped:
		out = append(out, c.Styles.Error.Render(c.Measure.Fit(m.head, w)), "")
		out = append(out, m.pagerView(w, max(0, h-2), c, false)...)
	}
	if len(out) > h {
		out = out[:h]
	}
	return strings.Join(out, "\n")
}

// pagerView returns the h lines of the pager from its top. The first line
// of a plan draws as a header.
func (m *mode) pagerView(w, h int, c ui.Context, plan bool) []string {
	lines := m.pagerLines(w, c)
	top := max(0, min(m.top, len(lines)-h))
	out := make([]string, h)
	for i := range out {
		if j := top + i; j < len(lines) {
			out[i] = c.Measure.Fit(lines[j], w)
			if plan && j == 0 {
				out[i] = c.Styles.Header.Render(out[i])
			}
		}
	}
	return out
}

// formView returns the lines of the record form in a body of h lines. A
// body too short gives up, in order, the blank above the messages, the
// blank above the fields, the blank under the title, the now line and the
// title; the four fields and one line of messages stay.
func (m *mode) formView(w, h int, c ui.Context) []string {
	f := m.form
	keep := min(5, max(0, h-5)) // title, now, the blank under the title, above the fields, above the messages
	var out []string
	if keep >= 1 {
		out = append(out, c.Styles.Header.Render(c.Measure.Fit("record "+f.id+" "+f.title, w)))
	}
	if keep >= 3 {
		out = append(out, "")
	}
	if keep >= 2 {
		out = append(out, f.now())
	}
	if keep >= 4 {
		out = append(out, "")
	}
	for i := range fields {
		out = append(out, f.line(i, w, c))
	}
	if keep >= 5 {
		out = append(out, "")
	}
	room := max(1, h-len(out))
	switch {
	case m.messages != nil:
		msgs := wrap(m.messages, w-2, c.Measure)
		if len(msgs) > room {
			n := len(msgs) - (room - 1)
			msgs = append(msgs[:room-1], fmt.Sprintf("… %d more", n))
		}
		for _, l := range msgs {
			out = append(out, c.Styles.Error.Render(c.Measure.Fit("  "+l, w)))
		}
	case m.uncommitted:
		out = append(out, c.Styles.Muted.Render(c.Measure.Fit("  "+uncommittedLine, w)))
	}
	return out
}

// Status implements ui.Mode.
func (m *mode) Status(c ui.Context) string {
	switch m.stage {
	case stageForm:
		return m.name + " · " + m.form.status()
	case stagePlanning:
		return m.name + " · planning…"
	case stageApplying:
		return m.name + " · committing…"
	}
	n := len(m.pagerLines(c.Width, c))
	h := m.pagerHeight(c.Width, c.Height, c)
	top := max(0, min(m.top, n-h))
	return fmt.Sprintf("%s · lines %d–%d of %d", m.name, min(top+1, n), min(top+h, n), n)
}

// Keys implements ui.Mode: the help line of the stage.
func (m *mode) Keys() []key.Binding {
	switch m.stage {
	case stageForm:
		return []key.Binding{helpPlan, helpField, helpChoice, helpCancel}
	case stageConfirm:
		return []key.Binding{helpCommit, helpNo, helpScroll}
	case stageStopped:
		return []key.Binding{helpClose, helpScroll}
	}
	return nil
}
