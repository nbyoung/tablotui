package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	installed := &debug.BuildInfo{Main: debug.Module{Version: "v0.3.1"}}
	devel := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	cases := []struct {
		name   string
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"linker wins", "v1.2.3", installed, "v1.2.3"},
		{"module version when not linked", "dev", installed, "v0.3.1"},
		{"devel falls back", "dev", devel, "dev"},
		{"no build info falls back", "dev", nil, "dev"},
		{"empty linked reads build info", "", installed, "v0.3.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveVersion(c.linked, c.info); got != c.want {
				t.Errorf("resolveVersion(%q, %v) = %q, want %q", c.linked, c.info, got, c.want)
			}
		})
	}
}

func TestBanner(t *testing.T) {
	if got, want := banner("v1.0.0"), "tablotui v1.0.0"; got != want {
		t.Errorf("banner = %q, want %q", got, want)
	}
}
