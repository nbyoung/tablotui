package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func views(t *testing.T) Views {
	t.Helper()
	v, err := LoadViews(testdata)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func ids(s map[string]bool) []string {
	out := []string{}
	for k, v := range s {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func TestClassify(t *testing.T) {
	v := views(t)
	cases := []struct {
		email    string
		role     Role
		owner    bool
		known    bool
		assigned []string
	}{
		{"ada@example.org", Owner, true, true, []string{"a1c0", "9f31", "7b2e"}},
		{"ben@example.org", Contributor, false, true, []string{"4e2b", "c07d"}},
		{"dan@example.org", Contributor, false, true, []string{"3c5d"}},
		{"opus@example.org", Observer, false, true, nil},
		{"nobody@example.org", Observer, false, false, nil},
		{"", Observer, false, false, nil},
	}
	for _, c := range cases {
		p := Classify(v, c.email)
		if p.Role() != c.role || p.Owner != c.owner || p.Known != c.known || !reflect.DeepEqual(p.Assigned, c.assigned) {
			t.Errorf("%q: got %+v role %s", c.email, p, p.Role())
		}
	}
}

func TestOwnerStart(t *testing.T) {
	s := OwnerStart(views(t))
	if s.Mode != Owner || len(s.Expanded) != 0 {
		t.Errorf("owner must start collapsed at the root: %+v", s)
	}
	// next gates of the leaves: mockup, performance, implementation, defined
	if s.First != "undefined" || s.Last != "unit" {
		t.Errorf("window %s..%s", s.First, s.Last)
	}
}

func TestContributorStart(t *testing.T) {
	v := views(t)
	cases := []struct {
		email       string
		expanded    []string
		first, last string
	}{
		// 9f31 performance, 7b2e defined
		{"ada@example.org", []string{"4e2b", "a1c0"}, "undefined", "reliability"},
		// c07d implementation
		{"ben@example.org", []string{"4e2b", "a1c0"}, "design", "unit"},
		// 3c5d mockup
		{"dan@example.org", []string{"a1c0"}, "defined", "function"},
	}
	for _, c := range cases {
		s := ContributorStart(v, Classify(v, c.email))
		if s.Mode != Contributor || !reflect.DeepEqual(ids(s.Expanded), c.expanded) || s.First != c.first || s.Last != c.last {
			t.Errorf("%s: got %v %s..%s", c.email, ids(s.Expanded), s.First, s.Last)
		}
	}
}

func TestStrangerStartsAsOwnerDoes(t *testing.T) {
	v := views(t)
	for _, email := range []string{"nobody@example.org", "opus@example.org"} {
		p := Classify(v, email)
		s := StartFor(v, p, p.Role())
		o := OwnerStart(v)
		if s.Mode != Observer || len(s.Expanded) != 0 || s.First != o.First || s.Last != o.Last {
			t.Errorf("%s: %+v", email, s)
		}
	}
}

// The contextual tableau that tablo emits for ben agrees on the tree: its
// spine rows are the expanded parents and its corner rows the assigned tasks.
func TestAgreesWithContextualView(t *testing.T) {
	v := views(t)
	var c Contextual
	if err := load(testdata, "contextual-person-ben.json", &c); err != nil {
		t.Fatal(err)
	}
	spine, corner := map[string]bool{}, []string{}
	for _, r := range c.Rows {
		switch r.Role {
		case "spine":
			spine[r.ID] = true
		case "corner":
			corner = append(corner, r.ID)
		}
	}
	s := ContributorStart(v, Classify(v, c.Person))
	if !reflect.DeepEqual(ids(spine), ids(s.Expanded)) {
		t.Errorf("spine %v, expanded %v", ids(spine), ids(s.Expanded))
	}
	if !reflect.DeepEqual(corner, []string{"c07d"}) {
		t.Errorf("corner %v", corner)
	}
}

// The work queue's ready and waiting items name the next gates that the task
// assignment view marks next.
func TestAgreesWithQueue(t *testing.T) {
	v := views(t)
	for _, name := range []string{"ada", "dan"} {
		var q Queue
		if err := load(testdata, "queue-"+name+".json", &q); err != nil {
			t.Fatal(err)
		}
		var fromQueue []Next
		for _, it := range q.Items {
			if it.Kind == "work ready" || it.Kind == "work waiting" {
				fromQueue = append(fromQueue, Next{it.Task, it.Gate})
			}
		}
		got := Classify(v, q.Person).ContribNext
		less := func(s []Next) func(i, j int) bool {
			return func(i, j int) bool { return s[i].Task < s[j].Task }
		}
		sort.Slice(fromQueue, less(fromQueue))
		sort.Slice(got, less(got))
		if !reflect.DeepEqual(fromQueue, got) {
			t.Errorf("%s: queue %v, assignment %v", name, fromQueue, got)
		}
	}
}

func TestBenQueueIsEmptyYetHasNextGate(t *testing.T) {
	var q Queue
	if err := load(testdata, "queue-ben.json", &q); err != nil {
		t.Fatal(err)
	}
	if len(q.Items) != 0 {
		t.Fatal("the fixture changed")
	}
	// The queue cannot supply ben's gates; the global tableau's next_gate does.
	s := ContributorStart(views(t), Classify(views(t), "ben@example.org"))
	if s.First != "design" {
		t.Errorf("window from %s", s.First)
	}
}

func TestResolveEmail(t *testing.T) {
	old := gitUserEmail
	defer func() { gitUserEmail = old }()
	gitUserEmail = func() (string, error) { return "dan@example.org", nil }
	if e, err := resolveEmail(""); e != "dan@example.org" || err != nil {
		t.Errorf("default: %q %v", e, err)
	}
	if e, _ := resolveEmail("ada@example.org"); e != "ada@example.org" {
		t.Errorf("--as must win: %q", e)
	}
	gitUserEmail = func() (string, error) { return "", errors.New("unset") }
	if _, err := resolveEmail(""); err == nil {
		t.Error("an unset identity must report")
	}
}

func press(m tea.Model, keys ...string) Model {
	for _, k := range keys {
		m, _ = m.Update(keyMsg(k))
	}
	return m.(Model)
}

func TestInitialStateFollowsRole(t *testing.T) {
	v := views(t)
	ada := NewModel(v, "ada@example.org")
	if ada.Mode != Owner || len(ada.VisibleRows()) != 1 {
		t.Errorf("owner: %s, %d rows", ada.Mode, len(ada.VisibleRows()))
	}
	ben := NewModel(v, "ben@example.org")
	if ben.Mode != Contributor || len(ben.VisibleRows()) != 6 {
		t.Errorf("contributor: %s, %d rows", ben.Mode, len(ben.VisibleRows()))
	}
}

func TestSwitchKeepsHiddenColumns(t *testing.T) {
	v := views(t)
	m := NewModel(v, "ada@example.org") // owner, window undefined..unit
	m = press(m, "l", "x")              // hides defined
	if !m.Hidden["defined"] {
		t.Fatalf("hidden %v", m.Hidden)
	}
	before := m.View()

	m = press(m, "r") // to her contributor state
	if m.Mode != Contributor || !reflect.DeepEqual(ids(m.Expanded), []string{"4e2b", "a1c0"}) || m.Last != "reliability" {
		t.Errorf("contributor state: %+v", m)
	}
	if !reflect.DeepEqual(ids(m.Hidden), []string{"defined"}) {
		t.Errorf("hidden after switch: %v", ids(m.Hidden))
	}
	if cols := m.VisibleColumns(); !reflect.DeepEqual(cols, []string{"undefined", "mockup", "function", "performance", "reliability"}) {
		t.Errorf("columns %v", cols)
	}
	if header := strings.Split(m.Render(false), "\n")[1]; strings.Contains(header, "defin") {
		t.Error("the hidden column shows after the switch")
	}

	m = press(m, "r") // back to the owner
	if m.Mode != Owner || len(m.Expanded) != 0 || m.First != "undefined" || m.Last != "unit" {
		t.Errorf("owner state: %+v", m)
	}
	if m.View() != before {
		t.Errorf("round trip changed the view:\n%s\n%s", before, m.View())
	}
}

func TestRolesHeld(t *testing.T) {
	v := views(t)
	cases := map[string][]Role{
		"ada@example.org":    {Owner, Contributor},
		"ben@example.org":    {Contributor},
		"opus@example.org":   {Observer},
		"nobody@example.org": {Observer},
	}
	for email, want := range cases {
		if got := Classify(v, email).Roles(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v", email, got)
		}
	}
}

func TestStrangerStaysObserver(t *testing.T) {
	m := NewModel(views(t), "nobody@example.org")
	before := m.Render(false)
	m = press(m, "r", "r")
	if m.Mode != Observer || m.Render(false) != before {
		t.Errorf("mode %s", m.Mode)
	}
	if !strings.Contains(m.View(), "one role only: observer") {
		t.Errorf("the display must say so:\n%s", m.View())
	}
	if strings.Contains(before, "role: owner") {
		t.Error("an observer must not read as owner")
	}
	if press(m, "j").Notice != "" {
		t.Error("the notice must clear on the next key")
	}
}

func TestContributorCannotBecomeOwner(t *testing.T) {
	m := NewModel(views(t), "ben@example.org")
	before := m.Render(false)
	m = press(m, "r")
	if m.Mode != Contributor || m.Render(false) != before {
		t.Errorf("mode %s", m.Mode)
	}
	if !strings.Contains(m.View(), "one role only: contributor") {
		t.Error("the display must say so")
	}
}

func TestExpandAndCollapseAll(t *testing.T) {
	m := NewModel(views(t), "ada@example.org")
	m = press(m, "E")
	if len(m.VisibleRows()) != 6 || !reflect.DeepEqual(ids(m.Expanded), []string{"4e2b", "a1c0"}) {
		t.Errorf("E: %d rows %v", len(m.VisibleRows()), ids(m.Expanded))
	}
	m = press(m, "x", "C")
	if len(m.VisibleRows()) != 1 || len(m.Hidden) != 1 {
		t.Errorf("C: %d rows, hidden %v", len(m.VisibleRows()), m.Hidden)
	}
	if m = press(NewModel(views(t), "dan@example.org"), "C", "E"); len(m.VisibleRows()) != 6 {
		t.Error("E after C must show every task")
	}
}

func TestDumpAndChrome(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"--as", "dan@example.org", "--dump", "--keys", "j,l"}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	data := out.String()
	for _, chrome := range []string{"window ", "expanded", "roles ", ">"} {
		if chrome != ">" && strings.Contains(data, chrome) || chrome == ">" && strings.Contains(data, "\n>") {
			t.Errorf("chrome %q in the data dump:\n%s", chrome, data)
		}
	}
	if !strings.Contains(data, "role: contributor") || !strings.Contains(data, "3c5d Dashboard") || strings.HasSuffix(data, "\n\n") {
		t.Errorf("data dump:\n%s", data)
	}
	for _, l := range strings.Split(data, "\n") {
		if l != strings.TrimRight(l, " ") {
			t.Errorf("trailing space: %q", l)
		}
	}
	out.Reset()
	run([]string{"--as", "dan@example.org", "--dump", "--chrome", "--keys", "j,l"}, &out, &errb)
	m := press(NewModel(views(t), "dan@example.org"), "j", "l")
	if out.String() != m.View() || !strings.Contains(out.String(), "window defined..function") {
		t.Errorf("chrome dump differs from View:\n%s", out.String())
	}
}

func TestRefusesSingleHyphenLongOption(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"-dump"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "--dump") {
		t.Errorf("code %d: %s", code, errb.String())
	}
	if err := checkArgs([]string{"--", "-dump"}); err != nil {
		t.Error("the check stops at --")
	}
	if err := checkArgs([]string{"--as", "-x", "a@b"}); err != nil {
		t.Error(err)
	}
}

func TestUsage(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"--help"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	u := errb.String()
	for _, want := range []string{"  --as\n", "  --keys\n", "  --dump\n", "  --chrome\n"} {
		if !strings.Contains(u, want) {
			t.Errorf("usage lacks %q:\n%s", want, u)
		}
	}
	if strings.Contains(u, "  -as") || strings.Contains(u, "  -dump") {
		t.Error("a long option shows with one hyphen")
	}
}

func TestExpandKey(t *testing.T) {
	m := NewModel(views(t), "ada@example.org")
	m = press(m, "enter")
	if rows := m.VisibleRows(); len(rows) != 4 { // root and its three children
		t.Errorf("rows %d", len(rows))
	}
	m = press(m, "enter")
	if len(m.VisibleRows()) != 1 {
		t.Error("enter must collapse again")
	}
}

func TestWindowSizeAndQuit(t *testing.T) {
	var m tea.Model = NewModel(views(t), "ada@example.org")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.(Model).width != 80 {
		t.Error("size not kept")
	}
	if _, cmd := m.Update(keyMsg("q")); cmd == nil {
		t.Error("q must quit")
	}
}

func TestParents(t *testing.T) {
	p := parents(views(t).Global)
	want := map[string]string{"4e2b": "a1c0", "9f31": "4e2b", "c07d": "4e2b", "7b2e": "a1c0", "3c5d": "a1c0"}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("%v", p)
	}
}

func TestFixturesParse(t *testing.T) {
	for _, n := range []string{"global-tableau", "authority-delegation", "task-assignment", "gate-definition"} {
		b, err := testdata.ReadFile("testdata/" + n + ".json")
		if err != nil || !json.Valid(b) {
			t.Errorf("%s: %v", n, err)
		}
	}
}
