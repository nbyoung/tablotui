package ui_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/ui"
)

// stub is a pane that draws its own marker and records what it receives.
type stub struct {
	id       string
	mark     string
	keys     []key.Binding
	got      *[]tea.Msg
	lastCtx  *ui.Context
	headline string
}

func newStub(id, mark string, keys ...key.Binding) stub {
	var got []tea.Msg
	var ctx ui.Context
	return stub{id: id, mark: mark, keys: keys, got: &got, lastCtx: &ctx}
}

func (s stub) ID() string { return s.id }
func (s stub) Init(c ui.Context) tea.Cmd {
	return c.Load(s.id, 7, func(context.Context) (any, error) { return s.id + " loaded", nil })
}
func (s stub) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	*s.got = append(*s.got, msg)
	*s.lastCtx = c
	return s, nil
}
func (s stub) View(w, h int, c ui.Context) string {
	lines := make([]string, h)
	for i := range lines {
		lines[i] = strings.Repeat(s.mark, w)
	}
	return strings.Join(lines, "\n")
}
func (s stub) Status(c ui.Context) string { return "status of " + s.id }
func (s stub) Keys() []key.Binding        { return s.keys }
func (s stub) Headline(c ui.Context) string {
	return s.headline
}

func (s stub) count(typ string) int {
	n := 0
	for _, m := range *s.got {
		if fmt.Sprintf("%T", m) == typ {
			n++
		}
	}
	return n
}

// run drives a model with messages and executes its commands as the program does.
type run struct {
	t    *testing.T
	m    tea.Model
	quit bool
}

func (r *run) send(msg tea.Msg) {
	r.t.Helper()
	m, cmd := r.m.Update(msg)
	r.m = m
	r.exec(cmd)
}

func (r *run) exec(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			r.exec(c)
		}
	case tea.QuitMsg:
		r.quit = true
	default:
		r.send(msg)
	}
}

func (r *run) key(code rune, text string, mod tea.KeyMod) {
	r.t.Helper()
	r.send(tea.KeyPressMsg{Code: code, Text: text, Mod: mod})
}

func (r *run) lines() []string {
	return strings.Split(r.m.(ui.Model).View().Content, "\n")
}

func split(w, h int, open []string, focus string) []ui.Rect {
	if len(open) < 2 {
		return []ui.Rect{{Pane: open[0], W: w, H: h}}
	}
	return []ui.Rect{{Pane: open[0], X: 0, Y: 0, W: w / 2, H: h}, {Pane: open[1], X: w / 2, Y: 0, W: w - w/2, H: h}}
}

func twoPanes(t *testing.T) (*run, stub, stub) {
	t.Helper()
	home, side := newStub("home", "h"), newStub("side", "s")
	home.headline = "Home headline"
	m := ui.New(ui.Options{Panes: []ui.Pane{home, side}, Layout: split})
	r := &run{t: t, m: m}
	r.send(tea.WindowSizeMsg{Width: 61, Height: 12})
	return r, home, side
}

// T3: below the floor the frame draws one line and no pane.
func TestFloor(t *testing.T) {
	home := newStub("home", "h")
	for w := 0; w <= 45; w++ {
		for h := 0; h <= 12; h++ {
			if w >= 40 && h >= 8 {
				continue
			}
			var m tea.Model = ui.New(ui.Options{Panes: []ui.Pane{home}})
			m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			c := m.View().Content
			if strings.Contains(c, "\n") {
				t.Fatalf("%dx%d: %d lines, want one", w, h, strings.Count(c, "\n")+1)
			}
			if strings.Contains(c, "h") && strings.Contains(c, "hh") {
				t.Errorf("%dx%d: draws the pane", w, h)
			}
			if want := fmt.Sprintf("tablotui needs 40×8, has %d×%d", w, h); w >= len([]rune(want)) && strings.TrimRight(c, " ") != want {
				t.Errorf("%dx%d: %q, want %q", w, h, c, want)
			}
			if got := ansi.StringWidth(c); w > 0 && got != w {
				t.Errorf("%dx%d: line is %d cells wide", w, h, got)
			}
		}
	}
}

func TestViewIsAnAltScreenOfExactLines(t *testing.T) {
	r, _, _ := twoPanes(t)
	v := r.m.View()
	if !v.AltScreen {
		t.Error("AltScreen is not set")
	}
	lines := r.lines()
	if len(lines) != 12 {
		t.Fatalf("%d lines, want 12", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 61 {
			t.Errorf("line %d is %d cells wide, want 61", i, w)
		}
	}
	if !strings.HasPrefix(lines[0], "Home headline") {
		t.Errorf("context line = %q", lines[0])
	}
	if !strings.HasPrefix(lines[10], "status of home") {
		t.Errorf("status line = %q", lines[10])
	}
}

// T12: the frame, with a stub second pane and a two-rectangle layout.
func TestFrameLayoutAndFocus(t *testing.T) {
	r, home, side := twoPanes(t)
	// Only the home pane is open: it takes the whole body.
	if l := r.lines()[1]; strings.Contains(l, "s") || strings.Count(l, "h") != 61 {
		t.Errorf("one pane open: body line %q", l)
	}
	r.send(ui.ShowPaneMsg{Pane: "side", Show: true})
	for i, l := range r.lines()[1:10] {
		if want := strings.Repeat("h", 30) + strings.Repeat("s", 31); l != want {
			t.Errorf("body line %d = %q, want the two rectangles to tile it", i, l)
		}
	}
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of home") {
		t.Errorf("status = %q", got)
	}
	r.key(tea.KeyTab, "", 0)
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of side") {
		t.Errorf("after tab: status = %q", got)
	}
	r.key(tea.KeyTab, "", 0)
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of home") {
		t.Errorf("after the second tab: status = %q", got)
	}
	r.key(tea.KeyTab, "", tea.ModShift)
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of side") {
		t.Errorf("after shift+tab: status = %q", got)
	}
	r.key(tea.KeyEscape, "", 0)
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of home") {
		t.Errorf("after esc: status = %q", got)
	}
	// The focused pane has focus in its context; the other does not.
	r.send(ui.FocusMsg{Pane: "side"})
	r.send(ui.ReloadMsg{})
	if !side.lastCtx.Focused || home.lastCtx.Focused {
		t.Errorf("focus: side %v, home %v", side.lastCtx.Focused, home.lastCtx.Focused)
	}
	if side.lastCtx.Width != 31 || side.lastCtx.Height != 9 || home.lastCtx.Width != 30 {
		t.Errorf("sizes: side %dx%d, home %dx%d", side.lastCtx.Width, side.lastCtx.Height, home.lastCtx.Width, home.lastCtx.Height)
	}
	// Closing the focused pane focuses the home pane.
	r.send(ui.ShowPaneMsg{Pane: "side", Show: false})
	if got := r.lines()[10]; !strings.HasPrefix(got, "status of home") {
		t.Errorf("after closing the focused pane: status = %q", got)
	}
	// The home pane stays open.
	r.send(ui.ShowPaneMsg{Pane: "home", Show: false})
	if l := r.lines()[1]; strings.Count(l, "h") != 61 {
		t.Errorf("home closed: body line %q", l)
	}
}

func TestKeysGoToTheFocusedPaneAlone(t *testing.T) {
	r, home, side := twoPanes(t)
	r.send(ui.ShowPaneMsg{Pane: "side", Show: true})
	r.key('z', "z", 0)
	if home.count("tea.KeyPressMsg") != 1 || side.count("tea.KeyPressMsg") != 0 {
		t.Errorf("key counts: home %d, side %d", home.count("tea.KeyPressMsg"), side.count("tea.KeyPressMsg"))
	}
	r.key(tea.KeyTab, "", 0)
	r.key('z', "z", 0)
	if home.count("tea.KeyPressMsg") != 1 || side.count("tea.KeyPressMsg") != 1 {
		t.Errorf("key counts after tab: home %d, side %d", home.count("tea.KeyPressMsg"), side.count("tea.KeyPressMsg"))
	}
}

func TestLoadedMsgReachesItsPaneAlone(t *testing.T) {
	r, home, side := twoPanes(t)
	r.exec(r.m.Init())
	if home.count("ui.LoadedMsg") != 1 || side.count("ui.LoadedMsg") != 1 {
		t.Fatalf("each pane receives its own LoadedMsg: home %d, side %d", home.count("ui.LoadedMsg"), side.count("ui.LoadedMsg"))
	}
	if lm := (*home.got)[0].(ui.LoadedMsg); lm.Pane != "home" || lm.Seq != 7 || lm.Value != "home loaded" {
		t.Errorf("home got %+v", lm)
	}
	r.send(ui.LoadedMsg{Pane: "side", Seq: 1, Value: "x"})
	if home.count("ui.LoadedMsg") != 1 || side.count("ui.LoadedMsg") != 2 {
		t.Errorf("a LoadedMsg for side reached home: home %d, side %d", home.count("ui.LoadedMsg"), side.count("ui.LoadedMsg"))
	}
	r.send(ui.LoadedMsg{Pane: "nobody"})
	if home.count("ui.LoadedMsg") != 1 || side.count("ui.LoadedMsg") != 2 {
		t.Error("a LoadedMsg for no pane reached a pane")
	}
}

func TestLoadCarriesAnError(t *testing.T) {
	var got []tea.Msg
	p := loader{got: &got}
	var m tea.Model = ui.New(ui.Options{Panes: []ui.Pane{p}, Context: context.Background()})
	m, cmd := m.Update(tea.WindowSizeMsg{Width: 50, Height: 10})
	_ = cmd
	cmd = m.Init()
	m.Update(cmd())
	if len(got) != 1 {
		t.Fatalf("got %d messages", len(got))
	}
	if lm := got[0].(ui.LoadedMsg); lm.Err == nil || lm.Err.Error() != "boom" {
		t.Errorf("LoadedMsg = %+v", lm)
	}
}

type loader struct{ got *[]tea.Msg }

func (loader) ID() string { return "loader" }
func (loader) Init(c ui.Context) tea.Cmd {
	return c.Load("loader", 1, func(context.Context) (any, error) { return nil, errors.New("boom") })
}
func (l loader) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	*l.got = append(*l.got, msg)
	return l, nil
}
func (loader) View(w, h int, c ui.Context) string { return "" }
func (loader) Status(c ui.Context) string         { return "" }
func (loader) Keys() []key.Binding                { return nil }

func TestSelectionBroadcastAndCommand(t *testing.T) {
	var ran []ui.SelectionMsg
	home, side := newStub("home", "h"), newStub("side", "s")
	cmd := ui.Command{
		Binding: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run")),
		Run: func(sel ui.SelectionMsg) tea.Cmd {
			ran = append(ran, sel)
			return func() tea.Msg { return ui.NoticeMsg{Text: "ran " + sel.Task} }
		},
	}
	r := &run{t: t, m: ui.New(ui.Options{Panes: []ui.Pane{home, side}, Commands: []ui.Command{cmd}, Layout: split})}
	r.send(tea.WindowSizeMsg{Width: 60, Height: 12})
	r.key('r', "r", 0)
	if len(ran) != 1 || ran[0] != (ui.SelectionMsg{}) {
		t.Fatalf("a command before any selection runs with %+v", ran)
	}
	r.send(ui.SelectionMsg{Task: "c2ad", Gate: "design"})
	if side.count("ui.SelectionMsg") != 1 || home.count("ui.SelectionMsg") != 1 {
		t.Errorf("a SelectionMsg reaches every pane: home %d, side %d", home.count("ui.SelectionMsg"), side.count("ui.SelectionMsg"))
	}
	r.send(ui.SelectionMsg{Task: "bc63"})
	r.key('r', "r", 0)
	if len(ran) != 2 || ran[1] != (ui.SelectionMsg{Task: "bc63"}) {
		t.Errorf("the command ran with %+v, want the latest selection", ran)
	}
	if got := strings.TrimRight(r.lines()[10], " "); got != "ran bc63" {
		t.Errorf("notice = %q", got)
	}
	// The command's key never reaches the pane.
	if home.count("tea.KeyPressMsg") != 0 {
		t.Error("a command key reached the pane")
	}
}

func TestNoticeLastsUntilTheNextKey(t *testing.T) {
	r, _, _ := twoPanes(t)
	r.send(ui.NoticeMsg{Text: "hello"})
	if got := strings.TrimRight(r.lines()[10], " "); got != "hello" {
		t.Errorf("notice = %q", got)
	}
	r.send(ui.NoticeMsg{Text: "it broke", Err: true})
	if got := strings.TrimRight(r.lines()[10], " "); got != "error: it broke" {
		t.Errorf("error notice = %q", got)
	}
	r.key('z', "z", 0)
	if got := strings.TrimRight(r.lines()[10], " "); got != "status of home" {
		t.Errorf("after a key: status = %q", got)
	}
}

// modeStub is a mode that records its keys and closes on "c".
type modeStub struct{ got *[]string }

func (m modeStub) Update(msg tea.Msg, c ui.Context) (ui.Mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		*m.got = append(*m.got, k.String())
		if k.String() == "c" {
			return nil, nil
		}
	}
	return m, nil
}
func (m modeStub) View(w, h int, c ui.Context) string {
	lines := make([]string, h)
	for i := range lines {
		lines[i] = "mode"
	}
	return strings.Join(lines, "\n")
}
func (m modeStub) Status(c ui.Context) string { return "mode status" }
func (m modeStub) Keys() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "close"))}
}

func TestModeTakesKeys(t *testing.T) {
	var got []string
	r, home, _ := twoPanes(t)
	r.send(ui.OpenModeMsg{Mode: modeStub{&got}})
	if l := r.lines()[1]; !strings.HasPrefix(l, "mode") {
		t.Errorf("body = %q, want the mode", l)
	}
	if l := r.lines()[10]; !strings.HasPrefix(l, "mode status") {
		t.Errorf("status = %q", l)
	}
	if l := r.lines()[11]; !strings.HasPrefix(l, "c close  ? help  q quit") {
		t.Errorf("help line = %q", l)
	}
	r.key('q', "q", 0)
	r.key('?', "?", 0)
	r.key(tea.KeyTab, "", 0)
	if r.quit {
		t.Error("q quit the program while a mode was open")
	}
	if strings.Join(got, ",") != "q,?,tab" {
		t.Errorf("mode keys = %v", got)
	}
	if home.count("tea.KeyPressMsg") != 0 {
		t.Error("a key reached the pane under a mode")
	}
	r.key(tea.KeyEscape, "", 0)
	if l := r.lines()[1]; strings.HasPrefix(l, "mode") {
		t.Errorf("esc did not close the mode: %q", l)
	}
	if strings.Join(got, ",") != "q,?,tab" {
		t.Errorf("esc reached the mode: %v", got)
	}
	// The mode closes itself by returning nil.
	r.send(ui.OpenModeMsg{Mode: modeStub{&got}})
	r.key('c', "c", 0)
	if l := r.lines()[1]; strings.HasPrefix(l, "mode") {
		t.Errorf("the mode did not close: %q", l)
	}
	// ctrl+c quits through a mode.
	r.send(ui.OpenModeMsg{Mode: modeStub{&got}})
	r.key('c', "", tea.ModCtrl)
	if !r.quit {
		t.Error("ctrl+c did not quit through the mode")
	}
}

func TestQuitAndHelpMode(t *testing.T) {
	r, _, _ := twoPanes(t)
	r.key('?', "?", 0)
	body := strings.Join(r.lines()[1:10], "\n")
	for _, want := range []string{"Frame", "quit", "help", "next pane", "previous pane", "home pane"} {
		if !strings.Contains(body, want) {
			t.Errorf("help mode lacks %q:\n%s", want, body)
		}
	}
	r.key('?', "?", 0)
	if strings.Contains(strings.Join(r.lines()[1:10], "\n"), "Frame") {
		t.Error("? did not close the help mode")
	}
	r.key('q', "q", 0)
	if !r.quit {
		t.Error("q did not quit")
	}
	r2, _, _ := twoPanes(t)
	r2.key('c', "", tea.ModCtrl)
	if !r2.quit {
		t.Error("ctrl+c did not quit")
	}
}

// T13: the help mode lists every binding of the focused pane, by scope.
func TestHelpModeListsEveryBinding(t *testing.T) {
	bind := func(k, d string) key.Binding { return key.NewBinding(key.WithKeys(k), key.WithHelp(k, d)) }
	home := newStub("home", "h", bind("a", "alpha"), bind("b", "beta"), bind("c", "gamma"))
	cmd := ui.Command{Binding: bind("r", "run it"), Run: func(ui.SelectionMsg) tea.Cmd { return nil }}
	r := &run{t: t, m: ui.New(ui.Options{Panes: []ui.Pane{home}, Commands: []ui.Command{cmd}})}
	r.send(tea.WindowSizeMsg{Width: 60, Height: 30})
	r.key('?', "?", 0)
	body := strings.Join(r.lines(), "\n")
	for _, want := range []string{"alpha", "beta", "gamma", "run it", "Commands", "home", "quit", "next pane"} {
		if !strings.Contains(body, want) {
			t.Errorf("help mode lacks %q:\n%s", want, body)
		}
	}
}

// T13: the help line drops bindings from the end and keeps "? help  q quit".
func TestHelpLineDropsFromTheEnd(t *testing.T) {
	bind := func(k, d string) key.Binding { return key.NewBinding(key.WithKeys(k), key.WithHelp(k, d)) }
	home := newStub("home", "h", bind("a", "alpha"), bind("b", "beta"), bind("c", "gamma"), bind("d", "delta"))
	var m tea.Model = ui.New(ui.Options{Panes: []ui.Pane{home}})
	last := func(w int) string {
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 10})
		ls := strings.Split(m.View().Content, "\n")
		return strings.TrimRight(ls[len(ls)-1], " ")
	}
	cases := map[int]string{
		80: "a alpha  b beta  c gamma  d delta  ? help  q quit",
		49: "a alpha  b beta  c gamma  d delta  ? help  q quit",
		48: "a alpha  b beta  c gamma  ? help  q quit",
		40: "a alpha  b beta  c gamma  ? help  q quit",
	}
	for w, want := range cases {
		if got := last(w); got != want {
			t.Errorf("width %d: %q, want %q", w, got, want)
		}
	}
	none := ui.New(ui.Options{Panes: []ui.Pane{newStub("home", "h")}})
	none2, _ := none.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	ls := strings.Split(none2.View().Content, "\n")
	if got := strings.TrimRight(ls[len(ls)-1], " "); got != "? help  q quit" {
		t.Errorf("no bindings: %q", got)
	}
}

// T13: CheckKeys reports a key that two bindings share.
func TestCheckKeys(t *testing.T) {
	bind := func(ks ...string) key.Binding { return key.NewBinding(key.WithKeys(ks...), key.WithHelp(ks[0], "x")) }
	ok := newStub("ok", "o", bind("a"), bind("b", "c"))
	if err := ui.CheckKeys(ui.Options{Panes: []ui.Pane{ok}}); err != nil {
		t.Errorf("CheckKeys: %v", err)
	}
	cases := map[string]ui.Options{
		"within a pane":     {Panes: []ui.Pane{newStub("p", "p", bind("x"), bind("y", "x"))}},
		"frame key":         {Panes: []ui.Pane{newStub("p", "p", bind("q"))}},
		"frame key tab":     {Panes: []ui.Pane{newStub("p", "p", bind("tab"))}},
		"command and pane":  {Panes: []ui.Pane{newStub("p", "p", bind("x"))}, Commands: []ui.Command{{Binding: bind("x")}}},
		"command and frame": {Panes: []ui.Pane{ok}, Commands: []ui.Command{{Binding: bind("?")}}},
	}
	for name, o := range cases {
		if err := ui.CheckKeys(o); err == nil {
			t.Errorf("%s: CheckKeys found no clash", name)
		}
	}
	// Two panes may share a key: one has focus at a time.
	both := ui.Options{Panes: []ui.Pane{newStub("a", "a", bind("x")), newStub("b", "b", bind("x"))}}
	if err := ui.CheckKeys(both); err != nil {
		t.Errorf("CheckKeys: %v", err)
	}
}

// T14: with DefaultStyles the stripped view equals the plain view.
func TestStylesStripToPlain(t *testing.T) {
	home := newStub("home", "h")
	home.headline = "Headline"
	plain := ui.New(ui.Options{Panes: []ui.Pane{home}})
	styled := ui.New(ui.Options{Panes: []ui.Pane{home}, Styles: ui.DefaultStyles()})
	for _, m := range []*ui.Model{&plain, &styled} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 10})
		*m = next.(ui.Model)
	}
	p, s := plain.View().Content, styled.View().Content
	if strings.Contains(p, "\x1b") {
		t.Error("the zero Styles draw an escape sequence")
	}
	if !strings.Contains(s, "\x1b[") {
		t.Error("DefaultStyles draw no style")
	}
	if ansi.Strip(s) != p {
		t.Errorf("stripped styled view differs from the plain view:\n%q\n%q", ansi.Strip(s), p)
	}
}
