// Command f394 demonstrates how tablotui derives its starting state from the
// person at the keyboard: the owner at the root with the tree collapsed, a
// contributor at the parents of their tasks in a window around their next gates.
package main

import (
	"errors"
	"flag"
	"fmt"
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
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// runScript applies comma-separated key names and returns the model.
func runScript(m tea.Model, script string) tea.Model {
	for _, k := range strings.Split(script, ",") {
		if k != "" {
			m, _ = m.Update(keyMsg(k))
		}
	}
	return m
}

func main() {
	as := flag.String("as", "", "email of the person at the keyboard (default: git config user.email)")
	script := flag.String("script", "", "comma-separated keys to apply, then print the view and exit")
	dump := flag.Bool("dump", false, "print the view after the script and exit")
	flag.Parse()
	email, err := resolveEmail(*as)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tablotui:", err, "(starting as an observer)")
	}
	v, err := LoadViews(testdata)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tablotui:", err)
		os.Exit(1)
	}
	var m tea.Model = NewModel(v, email)
	if *dump || *script != "" {
		fmt.Print(runScript(m, *script).View())
		return
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tablotui:", err)
		os.Exit(1)
	}
}
