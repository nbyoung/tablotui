package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSingleHyphenLongOptionRefused(t *testing.T) {
	for _, a := range []string{"-dump", "-settings=x", "-strip-vs16"} {
		var out, errb bytes.Buffer
		if code := run([]string{a, files[0]}, &out, &errb); code != 2 {
			t.Errorf("%s: exit %d, want 2", a, code)
		}
		if !strings.Contains(errb.String(), "--"+strings.TrimPrefix(strings.SplitN(a, "=", 2)[0], "-")) {
			t.Errorf("%s: error does not name the -- form: %s", a, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"--dump", "--", "-x"}, &out, &errb); strings.Contains(errb.String(), "two hyphens") {
		t.Errorf("check did not stop at --: %d %s", code, errb.String())
	}
}

func TestUsageNamesDoubleHyphens(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--help"}, &out, &errb); code != 0 {
		t.Errorf("--help exit %d", code)
	}
	u := errb.String()
	for _, o := range []string{"--dump", "--keys", "--size", "--chrome", "--settings", "--strip-vs16"} {
		if !strings.Contains(u, o) {
			t.Errorf("usage lacks %s:\n%s", o, u)
		}
	}
	if strings.Contains(strings.ReplaceAll(u, "--", ""), "-dump") {
		t.Errorf("usage shows a single hyphen:\n%s", u)
	}
}

func TestDumpChrome(t *testing.T) {
	var out, errb bytes.Buffer
	args := []string{"--dump", "--size", "100x12", "--keys", "down,left", files[0]}
	if code := run(args, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	plain := out.String()
	for _, c := range []string{"window", "move", "columns ", "selected"} {
		if strings.Contains(plain, c) {
			t.Errorf("default dump holds chrome %q:\n%s", c, plain)
		}
	}
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	if len(lines) != 2+4 || strings.TrimSpace(lines[len(lines)-1]) == "" {
		t.Errorf("default dump has %d lines, want header, rule and 4 rows, no padding:\n%s", len(lines), plain)
	}
	out.Reset()
	if code := run(append([]string{"--chrome"}, args...), &out, &errb); code != 0 {
		t.Fatal(code)
	}
	m := size(NewModel(load(t, files[0]), ""), 100, 12)
	m = press(m, "down", "left")
	if want := m.View() + "\n"; out.String() != want {
		t.Errorf("--chrome differs from View():\n%s\nwant\n%s", out.String(), want)
	}
}
