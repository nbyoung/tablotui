package grid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
)

// rig drives the frame and the grid with messages: it starts no terminal.
type rig struct {
	t        *testing.T
	m        tea.Model
	quit     bool
	selected []ui.SelectionMsg
	notices  []ui.NoticeMsg
}

func fixtures() source.File { return source.File{Dir: "testdata"} }

// newRig builds the frame with the grid as its home pane, sizes it and runs
// the first load.
func newRig(t *testing.T, src source.Tableaux, settingsPath string, start Start, w, h int) *rig {
	t.Helper()
	g := New(Options{Source: src, Settings: settingsPath, Start: start})
	return newRigWith(t, ui.New(ui.Options{Panes: []ui.Pane{g}, Styles: ui.DefaultStyles()}), w, h)
}

func newRigWith(t *testing.T, m ui.Model, w, h int) *rig {
	t.Helper()
	r := &rig{t: t, m: m}
	r.send(tea.WindowSizeMsg{Width: w, Height: h})
	r.run(m.Init())
	return r
}

func (r *rig) send(msg tea.Msg) {
	r.t.Helper()
	switch msg := msg.(type) {
	case ui.SelectionMsg:
		r.selected = append(r.selected, msg)
	case ui.NoticeMsg:
		r.notices = append(r.notices, msg)
	}
	m, cmd := r.m.Update(msg)
	r.m = m
	r.run(cmd)
}

// run executes a command and feeds its message back, as the program does.
func (r *rig) run(cmd tea.Cmd) {
	r.t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			r.run(c)
		}
	case tea.QuitMsg:
		r.quit = true
	default:
		r.send(msg)
	}
}

var named = map[string]tea.KeyPressMsg{
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"left":      {Code: tea.KeyLeft},
	"right":     {Code: tea.KeyRight},
	"enter":     {Code: tea.KeyEnter},
	"pgup":      {Code: tea.KeyPgUp},
	"pgdown":    {Code: tea.KeyPgDown},
	"home":      {Code: tea.KeyHome},
	"end":       {Code: tea.KeyEnd},
	"space":     {Code: tea.KeySpace, Text: " "},
	"tab":       {Code: tea.KeyTab},
	"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
	"esc":       {Code: tea.KeyEscape},
	"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
}

func keyMsg(name string) tea.KeyPressMsg {
	if k, ok := named[name]; ok {
		return k
	}
	rs := []rune(name)
	if len(rs) != 1 {
		panic("unknown key " + name)
	}
	return tea.KeyPressMsg{Code: rs[0], Text: name}
}

// keys sends the space-separated key names.
func (r *rig) keys(script string) *rig {
	r.t.Helper()
	for _, k := range strings.Fields(script) {
		r.send(keyMsg(k))
	}
	return r
}

// content returns the drawn lines.
func (r *rig) content() []string { return strings.Split(r.m.(ui.Model).View().Content, "\n") }

// screen returns the view with ANSI and trailing spaces removed.
func (r *rig) screen() string {
	lines := r.content()
	for i, l := range lines {
		lines[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return strings.Join(lines, "\n")
}

func (r *rig) line(i int) string { return strings.TrimRight(ansi.Strip(r.content()[i]), " ") }

func (r *rig) status() string { return r.line(len(r.content()) - 2) }

// settingsFile writes a settings file under a temporary directory.
func settingsFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func loadSettings(t *testing.T, path string) map[string]settings.Choice {
	t.Helper()
	f, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f.Columns
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
