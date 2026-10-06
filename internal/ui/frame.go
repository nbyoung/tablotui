package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Pane is one region of the body. View returns exactly h lines of exactly w
// cells under c.Measure. Update receives the keys while the pane has focus,
// the LoadedMsg addressed to it, and every other message.
type Pane interface {
	ID() string
	Init(c Context) tea.Cmd
	Update(msg tea.Msg, c Context) (Pane, tea.Cmd)
	View(w, h int, c Context) string
	Status(c Context) string // the status line while the pane has focus
	Keys() []key.Binding     // for the help line and the help mode, in order
}

// Headliner is a home pane that supplies the context line.
type Headliner interface{ Headline(c Context) string }

// Mode takes the whole body and every key but ctrl+c and esc: a form, a
// confirmation, the help. Update returns nil to close the mode.
type Mode interface {
	Update(msg tea.Msg, c Context) (Mode, tea.Cmd)
	View(w, h int, c Context) string
	Status(c Context) string
	Keys() []key.Binding
}

// Command is a global key that acts on the selection: an action, a role switch.
type Command struct {
	Binding key.Binding
	Run     func(sel SelectionMsg) tea.Cmd
}

// Context is what the frame lends a pane or a mode on every call. Width and
// Height give the size of the rectangle the pane draws in, or of the body for
// a mode, so that a pane settles its scroll in Update without storing a width;
// both are 0 for a pane that no rectangle holds.
type Context struct {
	Styles  Styles
	Measure Measure
	Focused bool
	Width   int
	Height  int
	// Load runs f off the event loop and delivers its result as a LoadedMsg.
	Load func(pane string, seq uint64, f func(context.Context) (any, error)) tea.Cmd
}

// Rect places one open pane in the body. Layout returns rectangles that tile
// the body exactly, with no overlap and no gap.
type Rect struct {
	Pane       string
	X, Y, W, H int
}

// Layout places the open panes in a body of w by h cells.
type Layout func(w, h int, open []string, focus string) []Rect

// Options configures the frame. The first pane is the home pane: it is always
// open, opens focused, and esc returns to it. A nil Layout gives the home
// pane the whole body. A nil Context is context.Background.
type Options struct {
	Panes    []Pane
	Commands []Command
	Layout   Layout
	Styles   Styles
	Context  context.Context
}

// The smallest terminal the frame draws in.
const (
	minWidth  = 40
	minHeight = 8
	// chrome is the lines outside the body: the context, status and help lines.
	chrome = 3
)

// Model is the root model of the program. It implements tea.Model.
type Model struct {
	o       Options
	panes   []Pane
	open    []string
	focus   string
	mode    Mode
	w, h    int
	measure Measure
	sel     SelectionMsg
	notice  NoticeMsg
	keys    frameKeys
	ctx     context.Context
}

// New builds the root model.
func New(o Options) Model {
	m := Model{o: o, panes: slices.Clone(o.Panes), keys: newFrameKeys(), ctx: o.Context}
	if m.ctx == nil {
		m.ctx = context.Background()
	}
	if len(m.panes) > 0 {
		m.open = []string{m.panes[0].ID()}
		m.focus = m.panes[0].ID()
	}
	return m
}

// Init implements tea.Model: it initialises every pane.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, p := range m.panes {
		cmds = append(cmds, p.Init(m.context(p.ID())))
	}
	return tea.Batch(cmds...)
}

func (m Model) home() string {
	if len(m.panes) == 0 {
		return ""
	}
	return m.panes[0].ID()
}

func (m Model) bodyHeight() int { return max(0, m.h-chrome) }

func (m Model) tooSmall() bool { return m.w < minWidth || m.h < minHeight }

func (m Model) load(pane string, seq uint64, f func(context.Context) (any, error)) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		v, err := f(ctx)
		return LoadedMsg{Pane: pane, Seq: seq, Value: v, Err: err}
	}
}

// rects returns the rectangles of the open panes in the body.
func (m Model) rects() []Rect {
	w, h := m.w, m.bodyHeight()
	if m.o.Layout == nil {
		if m.home() == "" {
			return nil
		}
		return []Rect{{Pane: m.home(), W: w, H: h}}
	}
	return m.o.Layout(w, h, slices.Clone(m.open), m.focus)
}

func (m Model) context(pane string) Context {
	c := Context{
		Styles:  m.o.Styles,
		Measure: m.measure,
		Focused: m.mode == nil && pane == m.focus,
		Load:    m.load,
	}
	if m.tooSmall() {
		return c
	}
	for _, r := range m.rects() {
		if r.Pane == pane {
			c.Width, c.Height = r.W, r.H
			break
		}
	}
	return c
}

func (m Model) modeContext() Context {
	c := m.context("")
	c.Focused = true
	c.Width, c.Height = m.w, m.bodyHeight()
	return c
}

func (m Model) paneIndex(id string) int {
	return slices.IndexFunc(m.panes, func(p Pane) bool { return p.ID() == id })
}

func (m Model) isOpen(id string) bool { return slices.Contains(m.open, id) }

// Update implements tea.Model. It routes in this order: the size, the mode
// report, a key, then every other message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.ModeReportMsg:
		if msg.Mode == ansi.ModeUnicodeCore && (msg.Value == ansi.ModeSet || msg.Value == ansi.ModeReset || msg.Value == ansi.ModePermanentlySet) {
			m.measure.Method = ansi.GraphemeWidth
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.keyPress(msg)
	case LoadedMsg:
		return m.toPane(msg.Pane, msg)
	case SelectionMsg:
		m.sel = msg
		return m.broadcast(msg)
	case ReloadMsg:
		return m.broadcast(msg)
	case NoticeMsg:
		m.notice = msg
		return m, nil
	case OpenModeMsg:
		m.mode = msg.Mode
		return m, nil
	case ShowPaneMsg:
		return m.showPane(msg), nil
	case FocusMsg:
		return m.focusPane(msg.Pane), nil
	}
	return m.broadcast(msg)
}

// toPane delivers msg to the pane named id alone.
func (m Model) toPane(id string, msg tea.Msg) (tea.Model, tea.Cmd) {
	i := m.paneIndex(id)
	if i < 0 {
		return m, nil
	}
	p, cmd := m.panes[i].Update(msg, m.context(id))
	m.panes[i] = p
	return m, cmd
}

// broadcast delivers msg to every pane and to the mode.
func (m Model) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i, p := range m.panes {
		np, cmd := p.Update(msg, m.context(p.ID()))
		m.panes[i] = np
		cmds = append(cmds, cmd)
	}
	if m.mode != nil {
		mode, cmd := m.mode.Update(msg, m.modeContext())
		m.mode = mode
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) keyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.notice = NoticeMsg{}
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.mode != nil {
		if msg.String() == "esc" {
			m.mode = nil
			return m, nil
		}
		mode, cmd := m.mode.Update(msg, m.modeContext())
		m.mode = mode
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.mode = newHelpMode(m.helpSections())
		return m, nil
	case key.Matches(msg, m.keys.Next):
		return m.cycle(1), nil
	case key.Matches(msg, m.keys.Prev):
		return m.cycle(-1), nil
	case key.Matches(msg, m.keys.Home):
		m.focus = m.home()
		return m, nil
	}
	for _, c := range m.o.Commands {
		if key.Matches(msg, c.Binding) {
			if c.Run == nil {
				return m, nil
			}
			return m, c.Run(m.sel)
		}
	}
	return m.toPane(m.focus, msg)
}

func (m Model) cycle(step int) Model {
	if len(m.open) == 0 {
		return m
	}
	i := slices.Index(m.open, m.focus)
	m.focus = m.open[((i+step)%len(m.open)+len(m.open))%len(m.open)]
	return m
}

func (m Model) showPane(msg ShowPaneMsg) Model {
	if m.paneIndex(msg.Pane) < 0 {
		return m
	}
	open := slices.Clone(m.open)
	switch {
	case msg.Show && !slices.Contains(open, msg.Pane):
		open = append(open, msg.Pane)
	case !msg.Show && msg.Pane != m.home():
		open = slices.DeleteFunc(open, func(id string) bool { return id == msg.Pane })
		if m.focus == msg.Pane {
			m.focus = m.home()
		}
	}
	m.open = open
	return m
}

func (m Model) focusPane(id string) Model {
	if m.paneIndex(id) < 0 {
		return m
	}
	if !m.isOpen(id) {
		m.open = append(slices.Clone(m.open), id)
	}
	m.focus = id
	return m
}

func (m Model) helpSections() []section {
	s := []section{{Scope: "Frame", Keys: m.keys.all()}}
	if len(m.o.Commands) > 0 {
		cs := section{Scope: "Commands"}
		for _, c := range m.o.Commands {
			cs.Keys = append(cs.Keys, c.Binding)
		}
		s = append(s, cs)
	}
	if i := m.paneIndex(m.focus); i >= 0 {
		s = append(s, section{Scope: m.panes[i].ID(), Keys: m.panes[i].Keys()})
	}
	return s
}

// View implements tea.Model. Every line is exactly as wide as the terminal.
func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.SetContent(m.content())
	return v
}

func (m Model) content() string {
	if m.tooSmall() {
		return m.measure.Fit(fmt.Sprintf("tablotui needs %d×%d, has %d×%d", minWidth, minHeight, m.w, m.h), m.w)
	}
	lines := make([]string, 0, m.h)
	lines = append(lines, m.contextLine())
	lines = append(lines, m.body()...)
	lines = append(lines, m.statusLine(), m.helpLine())
	return strings.Join(lines, "\n")
}

func (m Model) contextLine() string {
	var s string
	if len(m.panes) > 0 {
		if h, ok := m.panes[0].(Headliner); ok {
			s = h.Headline(m.context(m.home()))
		}
	}
	return m.o.Styles.Title.Render(m.measure.Fit(s, m.w))
}

// body returns exactly bodyHeight lines of exactly w cells.
func (m Model) body() []string {
	h := m.bodyHeight()
	blank := strings.Repeat(" ", m.w)
	if m.mode != nil {
		return m.fit(strings.Split(m.mode.View(m.w, h, m.modeContext()), "\n"), m.w, h)
	}
	type seg struct {
		x, w int
		text string
	}
	rows := make([][]seg, h)
	for _, r := range m.rects() {
		i := m.paneIndex(r.Pane)
		if i < 0 || !m.isOpen(r.Pane) || r.W <= 0 {
			continue
		}
		x := max(0, r.X)
		w := min(r.W, m.w-x)
		if w <= 0 {
			continue
		}
		lines := m.fit(strings.Split(m.panes[i].View(r.W, r.H, m.context(r.Pane)), "\n"), r.W, r.H)
		for j, ln := range lines {
			if y := r.Y + j; y >= 0 && y < h {
				if w < r.W {
					ln = m.measure.Fit(ln, w)
				}
				rows[y] = append(rows[y], seg{x, w, ln})
			}
		}
	}
	out := make([]string, h)
	for y, segs := range rows {
		if len(segs) == 0 {
			out[y] = blank
			continue
		}
		slices.SortFunc(segs, func(a, b seg) int { return a.x - b.x })
		var b strings.Builder
		at := 0
		for _, s := range segs {
			if s.x < at {
				continue
			}
			b.WriteString(strings.Repeat(" ", s.x-at))
			b.WriteString(s.text)
			at = s.x + s.w
		}
		b.WriteString(strings.Repeat(" ", max(0, m.w-at)))
		out[y] = b.String()
	}
	return out
}

// fit forces lines to n lines of w cells each.
func (m Model) fit(lines []string, w, n int) []string {
	out := make([]string, n)
	for i := range out {
		if i < len(lines) {
			out[i] = m.measure.Fit(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}

func (m Model) statusLine() string {
	s := m.o.Styles
	if m.notice.Text != "" {
		text := m.notice.Text
		if m.notice.Err {
			return s.Error.Render(m.measure.Fit("error: "+text, m.w))
		}
		return s.Muted.Render(m.measure.Fit(text, m.w))
	}
	var text string
	switch {
	case m.mode != nil:
		text = m.mode.Status(m.modeContext())
	default:
		if i := m.paneIndex(m.focus); i >= 0 {
			text = m.panes[i].Status(m.context(m.focus))
		}
	}
	return s.Muted.Render(m.measure.Fit(text, m.w))
}

// tail ends every help line.
const helpTail = "? help  q quit"

func (m Model) helpLine() string {
	var bindings []key.Binding
	switch {
	case m.mode != nil:
		bindings = m.mode.Keys()
	default:
		if i := m.paneIndex(m.focus); i >= 0 {
			bindings = m.panes[i].Keys()
		}
	}
	var items []string
	for _, b := range bindings {
		h := b.Help()
		if !b.Enabled() || h.Key == "" && h.Desc == "" {
			continue
		}
		items = append(items, strings.TrimSpace(h.Key+" "+h.Desc))
	}
	line := helpTail
	for n := len(items); n > 0; n-- {
		cand := strings.Join(items[:n], "  ") + "  " + helpTail
		if m.measure.Width(cand) <= m.w {
			line = cand
			break
		}
	}
	return m.o.Styles.Muted.Render(m.measure.Fit(line, m.w))
}
