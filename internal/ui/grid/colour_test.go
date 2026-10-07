package grid

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/ui"
)

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// T14: the stripped view equals the plain view, the cursor cells carry
// reverse, and a parent carries bold.
func TestColour(t *testing.T) {
	plainGrid := New(Options{Source: fixtures(), Start: GlanceStart()})
	plain := newRigWith(t, ui.New(ui.Options{Panes: []ui.Pane{plainGrid}}), 100, 14)
	styled := newRig(t, fixtures(), "", GlanceStart(), 100, 14)
	for _, keys := range []string{"", "down enter right right", "E d down down right right right right right right"} {
		plain.keys(keys)
		styled.keys(keys)
		if got, want := styled.screen(), plain.screen(); got != want {
			t.Errorf("keys %q: the stripped view differs from the plain view:\n%s\n%s", keys, got, want)
		}
	}
	if strings.Contains(plain.m.(ui.Model).View().Content, "\x1b") {
		t.Error("the zero Styles draw an escape sequence")
	}

	r := newRig(t, fixtures(), "", GlanceStart(), 100, 14).keys("down right right")
	lines := r.content()
	const reverse, bold = "\x1b[7m", "\x1b[1m"
	hasParam := func(s, param string) int {
		n := 0
		for _, m := range sgr.FindAllStringSubmatch(s, -1) {
			for _, p := range strings.Split(m[1], ";") {
				if p == param {
					n++
				}
			}
		}
		return n
	}
	// The selected row: its id and task, and the cell where the cursor column crosses.
	row := lines[4]
	if !strings.Contains(row, reverse+"bc63") {
		t.Errorf("the selected id is not reversed: %q", row)
	}
	if n := hasParam(row, "7"); n != 3 {
		t.Errorf("the selected row has %d reversed cells, want the id, the task and the crossing cell: %q", n, row)
	}
	if !strings.Contains(lines[1], reverse+"📝") {
		t.Errorf("the cursor column's header is not reversed: %q", lines[1])
	}
	if n := hasParam(lines[1], "7"); n != 1 {
		t.Errorf("the header has %d reversed cells, want 1: %q", n, lines[1])
	}
	// A parent is bold: the root, not selected, and the selected parent with its reverse.
	if hasParam(lines[3], "1") != 1 || !strings.Contains(lines[3], bold) {
		t.Errorf("the root is not bold: %q", lines[3])
	}
	if strings.Contains(lines[5], bold) {
		t.Errorf("a leaf is bold: %q", lines[5])
	}
	if hasParam(row, "1") != 1 {
		t.Errorf("the selected parent is not bold: %q", row)
	}
	// Reverse is no colour: no foreground or background sequence appears.
	for _, l := range lines[:4] {
		if strings.Contains(l, "\x1b[3") || strings.Contains(l, "\x1b[4") {
			t.Errorf("a colour sequence in %q", l)
		}
	}
}

func TestActsCellsAreBracketedAndColoured(t *testing.T) {
	start := GlanceStart()
	start.Request = reqPerson
	r := newRig(t, fixtures(), "", start, 100, 16).keys("down enter down")
	var found bool
	for _, l := range r.content() {
		if strings.Contains(l, "[🤖👀]") {
			found = true
			if !strings.Contains(l, "\x1b[1;33m") && !strings.Contains(l, "\x1b[1m\x1b[33m") && !strings.Contains(l, "33") {
				t.Errorf("an acting cell has no colour: %q", l)
			}
		}
	}
	if !found {
		t.Error("no acting cell")
	}
}

// T14: a program run on buffers.
func TestProgramProfiles(t *testing.T) {
	cases := []struct {
		name       string
		profile    colorprofile.Profile
		wantReason bool
		wantStyle  bool
	}{
		{"ascii", colorprofile.Ascii, true, true},
		{"no tty", colorprofile.NoTTY, false, false},
		{"true colour", colorprofile.TrueColor, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := New(Options{Source: fixtures(), Start: GlanceStart()})
			var out bytes.Buffer
			p := tea.NewProgram(quitWhenLoaded{ui.New(ui.Options{Panes: []ui.Pane{g}, Styles: ui.DefaultStyles()})},
				tea.WithInput(nil), tea.WithOutput(&out), tea.WithColorProfile(c.profile), tea.WithWindowSize(100, 14),
				tea.WithoutSignals(), tea.WithEnvironment([]string{"TERM=xterm-256color"}))
			if _, err := p.Run(); err != nil {
				t.Fatal(err)
			}
			s := out.String()
			if !strings.Contains(ansi.Strip(s), "Tableaux tooling") {
				t.Fatalf("the program drew no tableau: %q", s)
			}
			if got := strings.Contains(s, "\x1b[7m") || strings.Contains(s, ";7m") || strings.Contains(s, "[7;"); got != c.wantReason {
				t.Errorf("reverse present = %v, want %v", got, c.wantReason)
			}
			// No SGR sequence for a colour under Ascii or NoTTY.
			if c.profile != colorprofile.TrueColor {
				for _, bad := range []string{"38;5;", "38;2;", "48;5;", "48;2;", "\x1b[33m", "\x1b[31m", ";33m", ";31m"} {
					if strings.Contains(s, bad) {
						t.Errorf("a colour sequence %q under %v", bad, c.name)
					}
				}
			}
			if !c.wantStyle && strings.Contains(s, "m\x1b[") && hasSGR(s) {
				t.Errorf("a style sequence under %v: %q", c.name, s)
			}
		})
	}
}

// hasSGR reports whether s holds a Select Graphic Rendition sequence.
func hasSGR(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == 0x1b && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] == ';' || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				return true
			}
		}
	}
	return false
}

// quitWhenLoaded wraps the frame and ends the program once the grid has its view.
type quitWhenLoaded struct{ ui.Model }

func (q quitWhenLoaded) Init() tea.Cmd { return q.Model.Init() }

func (q quitWhenLoaded) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := q.Model.Update(msg)
	q.Model = m.(ui.Model)
	if _, ok := msg.(ui.LoadedMsg); ok {
		return q, tea.Sequence(cmd, tea.Quit)
	}
	return q, cmd
}
