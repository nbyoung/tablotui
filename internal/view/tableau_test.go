package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeFixtures(t *testing.T) {
	files, err := filepath.Glob("../ui/grid/testdata/*.json")
	if err != nil || len(files) != 6 {
		t.Fatalf("fixtures = %v, %v", files, err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			r, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()
			tb, err := Decode(r)
			if err != nil {
				t.Fatal(err)
			}
			if tb.Project != "Tableaux tooling" || len(tb.Gates) != 10 {
				t.Errorf("project %q, %d gates", tb.Project, len(tb.Gates))
			}
		})
	}
}

func TestDecodeFixtureContent(t *testing.T) {
	r, err := os.Open("../ui/grid/testdata/tableau.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	tb, err := Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Rows) != 37 || tb.Rows[0].ID != "437e" || tb.Rows[0].Status.From.Kind != "rollup" {
		t.Errorf("rows = %d, first %+v", len(tb.Rows), tb.Rows[0])
	}
	if c := tb.Rows[2].Cells[5]; c.Kind != "status" || c.Symbols != "🟢" || len(c.Facts) != 4 {
		t.Errorf("cell = %+v", c)
	}
}

const gates = `"gates": [{"key":"a","symbol":"A","name":"A","window":true,"count":0},{"key":"b","symbol":"B","name":"B","window":true,"count":0}]`

func row(id string, depth int, cells string) string {
	return `{"id":"` + id + `","title":"t","depth":` + string(rune('0'+depth)) + `,"parent":false,"label":"","status":{},"cells":[` + cells + `]}`
}

const okCells = `{"kind":"status","symbols":"x"},{"kind":"marks","symbols":"y"}`

func TestDecodeRefusals(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"malformed", `{`, "tableau"},
		{"trailing data", `{} {}`, "after"},
		{"too few cells", `{` + gates + `,"rows":[` + row("r1", 0, `{"kind":"status","symbols":"x"}`) + `]}`, "1 cells for 2 gates"},
		{"too many cells", `{` + gates + `,"rows":[` + row("r1", 0, okCells+`,{"kind":"exempt"}`) + `]}`, "3 cells for 2 gates"},
		{"depth rises by two", `{` + gates + `,"rows":[` + row("r1", 0, okCells) + `,` + row("r2", 2, okCells) + `]}`, "rises from depth 0 to 2"},
		{"negative depth", `{` + gates + `,"rows":[{"id":"r1","depth":-1,"cells":[` + okCells + `]}]}`, "depth -1"},
		{"unknown kind", `{` + gates + `,"rows":[` + row("r1", 0, `{"kind":"status","symbols":"x"},{"kind":"blank"}`) + `]}`, `unknown kind "blank"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(c.in))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Decode error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestDecodeAcceptsDepthFallsAndRisesByOne(t *testing.T) {
	in := `{` + gates + `,"rows":[` + row("r1", 0, okCells) + `,` + row("r2", 1, okCells) + `,` + row("r3", 2, okCells) + `,` + row("r4", 0, okCells) + `]}`
	tb, err := Decode(strings.NewReader(in))
	if err != nil || len(tb.Rows) != 4 {
		t.Errorf("Decode = %d rows, %v", len(tb.Rows), err)
	}
}
