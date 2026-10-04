// Command f394 demonstrates how tablotui derives its starting state from the
// person at the keyboard: the owner at the root with the tree collapsed, a
// contributor at the parents of their tasks in a window around their next
// gates. Its tree, marks and keys are a stand-in for the grid (task 679b).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// gitUserEmail reads the Git identity. Tests replace it.
var gitUserEmail = func() (string, error) {
	out, err := exec.Command("git", "config", "user.email").Output()
	return strings.TrimSpace(string(out)), err
}

// resolveEmail names the person: the --as value, else the Git identity.
func resolveEmail(as string) (string, error) {
	if as != "" {
		return as, nil
	}
	email, err := gitUserEmail()
	if err != nil || email == "" {
		return "", errors.New("no --as and no git user.email")
	}
	return email, nil
}

// keyMsg turns a key name into a message.
func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "comma":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// runKeys applies comma-separated key names and returns the model.
func runKeys(m tea.Model, keys string) tea.Model {
	for _, k := range strings.Split(keys, ",") {
		if k != "" {
			m, _ = m.Update(keyMsg(k))
		}
	}
	return m
}

// checkArgs refuses a long option written with one hyphen, up to a bare --.
func checkArgs(args []string) error {
	for _, a := range args {
		if a == "--" {
			return nil
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			return fmt.Errorf("use --%s, not %s: long options take two hyphens", a[1:], a)
		}
	}
	return nil
}

func say(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

func sayf(w io.Writer, s string) { _, _ = fmt.Fprint(w, s) }

func newFlagSet(stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("tablotui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.String("as", "", "email of the person at the keyboard (default: git config user.email)")
	fs.String("keys", "", "comma-separated keys to send before the dump (k1,k2,...; comma is named comma)")
	fs.Bool("dump", false, "print the view after the keys and exit")
	fs.Bool("chrome", false, "include the interface's chrome in the dump")
	fs.Usage = func() { usage(fs, stderr) }
	return fs
}

// usage prints each long option as --name and each one-letter option as -x.
func usage(fs *flag.FlagSet, w io.Writer) {
	say(w, "Usage: tablotui [options]")
	fs.VisitAll(func(f *flag.Flag) {
		dash := "--"
		if len(f.Name) == 1 {
			dash = "-"
		}
		def := ""
		if f.DefValue != "" && f.DefValue != "false" {
			def = fmt.Sprintf(" (default %q)", f.DefValue)
		}
		_, _ = fmt.Fprintf(w, "  %s%s\n        %s%s\n", dash, f.Name, f.Usage, def)
	})
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := checkArgs(args); err != nil {
		say(stderr, "tablotui:", err)
		return 2
	}
	fs := newFlagSet(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	get := func(n string) string { return fs.Lookup(n).Value.String() }
	email, err := resolveEmail(get("as"))
	if err != nil {
		say(stderr, "tablotui:", err, "(starting as an observer)")
	}
	v, err := LoadViews(testdata)
	if err != nil {
		say(stderr, "tablotui:", err)
		return 1
	}
	var m tea.Model = NewModel(v, email)
	if get("dump") == "true" || get("keys") != "" {
		m = runKeys(m, get("keys"))
		sayf(stdout, m.(Model).Render(get("chrome") == "true"))
		return 0
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		say(stderr, "tablotui:", err)
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
