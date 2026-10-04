package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSingleHyphenLongOptionRefused(t *testing.T) {
	var out, errb bytes.Buffer
	if code := realMain([]string{"-dump"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "--dump") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	if checkArgs([]string{"-C", "x", "--dump", "--at=v1"}) != nil {
		t.Fatal("valid arguments refused")
	}
	if checkArgs([]string{"--", "-dump"}) != nil {
		t.Fatal("check did not stop at --")
	}
	if checkArgs([]string{"-at=v1"}) == nil {
		t.Fatal("-at=v1 accepted")
	}
}

func TestUsageText(t *testing.T) {
	var out, errb bytes.Buffer
	if code := realMain([]string{"--help"}, &out, &errb); code != 0 {
		t.Fatalf("code %d", code)
	}
	u := errb.String()
	for _, want := range []string{"  -C ", "--at ", "--follow", "--dump", "--chrome"} {
		if !strings.Contains(u, want) {
			t.Errorf("usage lacks %q", want)
		}
	}
	for _, bad := range []string{" -at", " -follow", " -dump", " -chrome"} {
		if strings.Contains(u, bad) {
			t.Errorf("usage has single-hyphen %q", bad)
		}
	}
}

func TestDumpWithAndWithoutChrome(t *testing.T) {
	dir := newRepo(t)
	var plain, full, errb bytes.Buffer
	if code := realMain([]string{"-C", dir, "--dump"}, &plain, &errb); code != 0 {
		t.Fatalf("code %d: %s", code, errb.String())
	}
	if code := realMain([]string{"-C", dir, "--dump", "--chrome"}, &full, &errb); code != 0 {
		t.Fatalf("code %d: %s", code, errb.String())
	}
	p, f := plain.String(), full.String()
	for _, want := range []string{"commit", "digest", "ref", "mode"} {
		if !strings.Contains(p, want) {
			t.Errorf("data lacks %q", want)
		}
	}
	for _, chrome := range []string{"tablotui", "reloads", "loaded", "q quits"} {
		if strings.Contains(p, chrome) {
			t.Errorf("default dump has chrome %q", chrome)
		}
		if !strings.Contains(f, chrome) {
			t.Errorf("--chrome dump lacks %q", chrome)
		}
	}
	if strings.HasSuffix(p, "\n\n") {
		t.Error("trailing blank padding")
	}
}
