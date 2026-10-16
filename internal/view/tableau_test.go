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
			if tb.Project.Title != "Tableaux tooling" || len(tb.Legend.Gates) != 10 || tb.Source.Ref != "main" {
				t.Errorf("project %q, %d gates, ref %q", tb.Project.Title, len(tb.Legend.Gates), tb.Source.Ref)
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
	if tb.Command != "tableau" || tb.Params.Level != "provenance" || tb.Params.Window == nil || *tb.Params.Window != 1 {
		t.Errorf("head = %q, %+v", tb.Command, tb.Params)
	}
	if len(tb.Rows) != 37 || tb.Rows[0].ID != "437e" || tb.Rows[0].Status.From.Kind != "rollup" || tb.Rows[0].Leaf.Task.ID != "99f0" {
		t.Errorf("rows = %d, first %+v", len(tb.Rows), tb.Rows[0])
	}
	if c := tb.Rows[2].Cells[5]; c.Kind != "status" || c.State != "nominal" || tb.Rows[2].StatusCommit == nil {
		t.Errorf("cell = %+v", c)
	}
	if c := tb.Columns[0]; !c.Folded || len(c.Gates) != 1 || c.Gates[0] != "undefined" {
		t.Errorf("first column = %+v", c)
	}
}

// wrap wraps data in an envelope of the tableau.
func wrap(data string) string {
	return `{"schema":"tablo/1","command":"tableau","params":{"window":1},"data":` + data + `}`
}

const legend = `"legend":{"gates":[{"key":"a","symbol":"A"},{"key":"b","symbol":"B"}]},"columns":[{"gates":["a"],"by_gate":[0]},{"gates":["b"],"by_gate":[0]}]`

func row(id string, depth int, cells string) string {
	return `{"id":"` + id + `","title":"t","depth":` + string(rune('0'+depth)) + `,"parent":false,"label":"","status":{},"cells":[` + cells + `]}`
}

const okCells = `{"kind":"status","state":"x"},{"kind":"marks","marks":["y"]}`

func TestDecodeRefusals(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"malformed", `{`, "tableau"},
		{"trailing data", `{} {}`, "after"},
		{"another command", `{"command":"queue","data":{}}`, `the command "queue"`},
		{"no data", `{"command":"context","data":null}`, "no data"},
		{"absent data", `{"command":"context"}`, "no data"},
		{"too few cells", wrap(`{` + legend + `,"rows":[` + row("r1", 0, `{"kind":"status"}`) + `]}`), "1 cells for 2 gates"},
		{"too many cells", wrap(`{` + legend + `,"rows":[` + row("r1", 0, okCells+`,{"kind":"exempt"}`) + `]}`), "3 cells for 2 gates"},
		{"depth rises by two", wrap(`{` + legend + `,"rows":[` + row("r1", 0, okCells) + `,` + row("r2", 2, okCells) + `]}`), "rises from depth 0 to 2"},
		{"negative depth", wrap(`{` + legend + `,"rows":[{"id":"r1","depth":-1,"cells":[` + okCells + `]}]}`), "depth -1"},
		{"unknown kind", wrap(`{` + legend + `,"rows":[` + row("r1", 0, `{"kind":"status"},{"kind":"historical"}`) + `]}`), `unknown kind "historical"`},
		{"a column out of order", wrap(`{"legend":{"gates":[{"key":"a"},{"key":"b"}]},"columns":[{"gates":["b"]},{"gates":["a"]}],"rows":[]}`), `gate "b" out of the legend's order`},
		{"a gate no column names", wrap(`{"legend":{"gates":[{"key":"a"},{"key":"b"}]},"columns":[{"gates":["a"]}],"rows":[]}`), "1 of 2 gates"},
		{"an empty column", wrap(`{"legend":{"gates":[{"key":"a"}]},"columns":[{"gates":[]}],"rows":[]}`), "names no gate"},
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
	in := wrap(`{` + legend + `,"rows":[` + row("r1", 0, okCells) + `,` + row("r2", 1, okCells) + `,` + row("r3", 2, okCells) + `,` + row("r4", 0, okCells) + `]}`)
	tb, err := Decode(strings.NewReader(in))
	if err != nil || len(tb.Rows) != 4 {
		t.Errorf("Decode = %d rows, %v", len(tb.Rows), err)
	}
}
