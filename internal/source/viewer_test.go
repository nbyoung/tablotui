package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFileTableauEchoes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tableau.json"), []byte(minimal), 0o600); err != nil {
		t.Fatal(err)
	}
	f := File{Dir: dir}
	ctx := context.Background()
	tb, err := f.Tableau(ctx, Request{View: "tableau", Window: 1, Viewer: "ada@example.org", Role: "owner"})
	if err != nil || tb.Viewer != "ada@example.org" || tb.Role != "owner" {
		t.Errorf("echo = %q, %q, %v", tb.Viewer, tb.Role, err)
	}
	tb, err = f.Tableau(ctx, Request{View: "tableau", Window: 1})
	if err != nil || tb.Viewer != "" || tb.Role != "" {
		t.Errorf("no echo = %q, %q, %v", tb.Viewer, tb.Role, err)
	}
	a := f.Name(Request{View: "tableau", Window: 1})
	b := f.Name(Request{View: "tableau", Window: 1, Viewer: "ada@example.org", Role: "owner"})
	if a != b {
		t.Errorf("Name = %s with a viewer and a role, %s without", b, a)
	}
}

func TestFileViewer(t *testing.T) {
	ctx := context.Background()
	observer := []string{"observer"}

	t.Run("no file", func(t *testing.T) {
		v, err := File{Dir: t.TempDir()}.Viewer(ctx, "", "ada@example.org")
		if err != nil || v.Email != "ada@example.org" || !reflect.DeepEqual(v.Roles, observer) {
			t.Errorf("Viewer = %+v, %v", v, err)
		}
	})

	dir := t.TempDir()
	body := `{"ada@example.org": ["owner", "assignee"], "empty@example.org": []}`
	if err := os.WriteFile(filepath.Join(dir, "viewers.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	f := File{Dir: dir}

	t.Run("listed", func(t *testing.T) {
		v, err := f.Viewer(ctx, "", "ada@example.org")
		if err != nil || !reflect.DeepEqual(v.Roles, []string{"owner", "assignee"}) {
			t.Errorf("Viewer = %+v, %v", v, err)
		}
	})
	t.Run("unlisted", func(t *testing.T) {
		v, err := f.Viewer(ctx, "", "eve@example.org")
		if err != nil || v.Email != "eve@example.org" || !reflect.DeepEqual(v.Roles, observer) {
			t.Errorf("Viewer = %+v, %v", v, err)
		}
	})
	t.Run("listed with no role", func(t *testing.T) {
		v, err := f.Viewer(ctx, "", "empty@example.org")
		if err != nil || !reflect.DeepEqual(v.Roles, observer) {
			t.Errorf("Viewer = %+v, %v", v, err)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		bad := t.TempDir()
		if err := os.WriteFile(filepath.Join(bad, "viewers.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := File{Dir: bad}.Viewer(ctx, "", "ada@example.org")
		if err == nil || !strings.Contains(err.Error(), "viewers.json") {
			t.Errorf("error = %v", err)
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		bad := t.TempDir()
		if err := os.Mkdir(filepath.Join(bad, "viewers.json"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := File{Dir: bad}.Viewer(ctx, "", "ada@example.org")
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			t.Errorf("error = %v", err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		c, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := f.Viewer(c, "", "ada@example.org"); !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v", err)
		}
	})
}
