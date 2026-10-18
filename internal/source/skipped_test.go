package source

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/view"
	"github.com/nbyoung/tablotui/internal/view/list"
	"github.com/nbyoung/tablotui/internal/watch"
)

// skippedKeysFile lists the keys of tablo's view data that tablotui's types
// have no place for, one "<view> <path>" per line, sorted, as tabloio's T17
// lists its own (tablo 493e and 886d, decision 16 b). A new key of tablo then
// shows as a change of this file.
const skippedKeysFile = "testdata/skipped-keys.txt"

// TestTheKeysTablotuiSkips derives each view tablotui reads over the corpus
// entry weather-station through the adapter, decodes it into the program's
// types, and compares their JSON with tablo's data as a projection of it.
// Each key of tablo's data that the types skip stands in skippedKeysFile,
// and each key there is one a view skips.
func TestTheKeysTablotuiSkips(t *testing.T) {
	dir := corpustest.Entry(t, "weather-station")
	src, _ := stored(t, dir, "", watch.State{})
	ctx := t.Context()

	type line struct {
		view tablo.View
		p    tablo.Params
	}
	window := func(n int) *int { return &n }
	tasks := []string{"a1c0", "3c5d", "4e2b", "7b2e", "9f31", "c07d"}
	people := []string{"ada@example.org", "ben@example.org", "dan@example.org"}
	var lines []line
	for _, level := range []tablo.Level{tablo.LevelGlance, tablo.LevelDetail, tablo.LevelProvenance} {
		for _, w := range []int{0, 1, 2} {
			lines = append(lines, line{tablo.Tableau, tablo.Params{Level: level, Window: window(w)}})
		}
		lines = append(lines,
			line{tablo.Tableau, tablo.Params{Level: level, Window: window(1), Historical: true}},
			line{tablo.Tableau, tablo.Params{Level: level, Window: window(1), Person: "ben@example.org", Viewer: "ben@example.org"}},
		)
		for _, p := range people {
			lines = append(lines,
				line{tablo.Context, tablo.Params{Level: level, Window: window(1), Person: p}},
				line{tablo.Queue, tablo.Params{Level: level, Viewer: p}},
			)
		}
		for _, id := range tasks {
			lines = append(lines,
				line{tablo.Context, tablo.Params{Level: level, Window: window(1), Task: id, Viewer: "ada@example.org", Role: tablo.RoleOwner}},
				line{tablo.Task, tablo.Params{Level: level, Task: id}},
				line{tablo.Blockage, tablo.Params{Level: level, Task: id}},
				line{tablo.History, tablo.Params{Level: level, Task: id}},
			)
		}
		lines = append(lines,
			line{tablo.Blockage, tablo.Params{Level: level}},
			line{tablo.History, tablo.Params{Level: level}},
		)
	}

	skipped := map[string]bool{}
	for _, l := range lines {
		b, _, err := src.view(ctx, l.view, l.p)
		if err != nil {
			t.Errorf("%s %+v: %v", l.view, l.p, err)
			continue
		}
		got, err := decoded(l.view, b)
		if err != nil {
			t.Errorf("%s %+v: %v", l.view, l.p, err)
			continue
		}
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(b, &env); err != nil {
			t.Fatal(err)
		}
		unread := map[string]bool{}
		if d := agree(normalise(t, got), normalise(t, env.Data), "", unread); d != "" {
			t.Errorf("%s %+v: the types and tablo's data differ at %s", l.view, l.p, d)
		}
		for k := range unread {
			skipped[string(l.view)+" "+k] = true
		}
	}
	checkSkippedKeys(t, skipped)
}

// decoded decodes the envelope b of view v into the program's type and
// returns the type's JSON.
func decoded(v tablo.View, b []byte) ([]byte, error) {
	var out any
	switch v {
	case tablo.Tableau, tablo.Context:
		tb, err := view.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		out = tb
	default:
		var lv list.View
		switch v {
		case tablo.Task:
			lv = &list.Task{}
		case tablo.Queue:
			lv = &list.Queue{}
		case tablo.Blockage:
			lv = &list.Blockage{}
		case tablo.History:
			lv = &list.History{}
		}
		if err := list.Decode(bytes.NewReader(b), lv); err != nil {
			return nil, err
		}
		out = lv
	}
	return json.Marshal(out)
}

// checkSkippedKeys compares the keys the views skip with skippedKeysFile: a
// skipped key the file does not list fails, and so does a listed key that
// no view skips, or a file that is not sorted.
func checkSkippedKeys(t *testing.T, skipped map[string]bool) {
	t.Helper()
	raw, err := os.ReadFile(skippedKeysFile)
	if err != nil {
		t.Fatal(err)
	}
	listed := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if !sort.StringsAreSorted(listed) {
		t.Errorf("%s is not sorted", skippedKeysFile)
	}
	inFile := map[string]bool{}
	for _, k := range listed {
		inFile[k] = true
		if !skipped[k] {
			t.Errorf("%s lists %q, which no view skips", skippedKeysFile, k)
		}
	}
	var unlisted []string
	for k := range skipped {
		if !inFile[k] {
			unlisted = append(unlisted, k)
		}
	}
	sort.Strings(unlisted)
	for _, k := range unlisted {
		t.Errorf("tablotui skips %q, which %s does not list", k, skippedKeysFile)
	}
	perView := map[string]int{}
	for k := range skipped {
		v, _, _ := strings.Cut(k, " ")
		perView[v]++
	}
	t.Logf("tablotui skips %d keys of tablo's data: %v", len(skipped), perView)
}

// agree compares the JSON of the program's type with tablo's data, both
// decoded generically, and returns the path of the first difference, or "".
// It reads the type as a projection of tablo's data: a key the type does
// not carry is noted in unread and left out; a key tablo omits must hold a
// zero value in the type; a null and an empty list are equal.
func agree(got, want any, path string, unread map[string]bool) string {
	switch g := got.(type) {
	case map[string]any:
		w, ok := want.(map[string]any)
		if !ok {
			if isZero(got) && isZero(want) {
				return ""
			}
			return path
		}
		for k, gv := range g {
			wv, present := w[k]
			if !present {
				if !isZero(gv) {
					return path + "/" + k
				}
				continue
			}
			if d := agree(gv, wv, path+"/"+k, unread); d != "" {
				return d
			}
		}
		for k := range w {
			if _, ok := g[k]; !ok {
				unread[strings.TrimPrefix(path+"/"+k, "/")] = true
			}
		}
		return ""
	case []any:
		w, ok := want.([]any)
		if !ok {
			if isZero(got) && isZero(want) {
				return ""
			}
			return path
		}
		if len(g) != len(w) {
			return path
		}
		for i := range g {
			if d := agree(g[i], w[i], path+"/*", unread); d != "" {
				return d + "[" + strconv.Itoa(i) + "]"
			}
		}
		return ""
	}
	if reflect.DeepEqual(got, want) || (isZero(got) && isZero(want)) {
		return ""
	}
	return path
}

// isZero reports whether v is JSON null, an empty list or object, "", 0 or
// false, or a list or object that holds only such values.
func isZero(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case float64:
		return x == 0
	case bool:
		return !x
	case []any:
		for _, e := range x {
			if !isZero(e) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, e := range x {
			if !isZero(e) {
				return false
			}
		}
		return true
	}
	return false
}

// normalise decodes JSON into generic values, which compare without regard
// to key order or spacing.
func normalise(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
