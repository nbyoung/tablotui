// Verbatim copy of write.go from tabloio's prototype b618, taken from tabloio
// commit f502ddd ("Add the write commands prototype") on trunk, path
// prototype/b618/write.go. Only the package clause changes, from b618 to main.
// Do not edit: tabloio has no release, so this copy stands in for the exported
// package that tablotui will import or replace with the tabloio binary.

// Package b618 is the functional prototype of tabloio's write commands.
//
// Each command builds a Commit: a message with its trailers and the files to
// write. Show prints the commit before it exists; Apply writes the files as
// text and runs git commit. No YAML library touches a file: a status or task
// file changes line by line, so every other byte stays as it was.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Trailer is one line of the commit's final paragraph.
type Trailer struct{ Key, Value string }

// Commit is a planned commit: what Show displays and Apply makes.
type Commit struct {
	Subject  string
	Trailers []Trailer
	// Files maps a repository-relative path to its complete new content.
	Files map[string]string
}

// Agent names the model that composes a commit, for the Model and
// Co-Authored-By trailers. The zero value adds neither.
type Agent struct{ Model, Name string }

func (a Agent) trailers() []Trailer {
	if a.Model == "" {
		return nil
	}
	ts := []Trailer{{"Model", a.Model}}
	if a.Name != "" {
		ts = append(ts, Trailer{"Co-Authored-By", a.Name + " <noreply@anthropic.com>"})
	}
	return ts
}

// Message returns the commit message: the subject, then one paragraph of
// trailers with no blank line between them.
func (c Commit) Message() string {
	var b strings.Builder
	b.WriteString(c.Subject + "\n")
	if len(c.Trailers) > 0 {
		b.WriteString("\n")
		for _, t := range c.Trailers {
			b.WriteString(t.Key + ": " + t.Value + "\n")
		}
	}
	return b.String()
}

var (
	idRe   = regexp.MustCompile(`^[0-9a-f]{4}$`)
	keyRe  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	gateRe = regexp.MustCompile(`\{\s*key:\s*([a-z][a-z0-9_-]*)`)
)

func taskPath(id string) string   { return filepath.Join(".tableaux", "tasks", id+".yaml") }
func statusPath(id string) string { return filepath.Join(".tableaux", "status", id+".yaml") }

func checkTask(repo, id string) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("%q is not a task id", id)
	}
	if _, err := os.Stat(filepath.Join(repo, taskPath(id))); err != nil {
		return fmt.Errorf("no task %s in the project", id)
	}
	return nil
}

func checkGate(repo, gate string) error {
	b, err := os.ReadFile(filepath.Join(repo, ".tableaux", "gates.yaml"))
	if err != nil {
		return err
	}
	for _, m := range gateRe.FindAllStringSubmatch(string(b), -1) {
		if m[1] == gate {
			return nil
		}
	}
	return fmt.Errorf("gate %q is not in gates.yaml", gate)
}

func multi(repo string, ids []string, key string, gate string, a Agent) (Commit, error) {
	if len(ids) == 0 {
		return Commit{}, errors.New("no task named")
	}
	var ts []Trailer
	for _, id := range ids {
		if err := checkTask(repo, id); err != nil {
			return Commit{}, err
		}
		v := id
		if gate != "" {
			v += " " + gate
		}
		ts = append(ts, Trailer{key, v})
	}
	return Commit{Trailers: append(ts, a.trailers()...)}, nil
}

// Authorise composes an empty commit that accepts each task as it stands.
func Authorise(repo string, ids []string, a Agent) (Commit, error) {
	c, err := multi(repo, ids, "Authorised", "", a)
	c.Subject = "Accept " + plural(ids)
	return c, err
}

// Reaffirm composes an empty commit that confirms each status as it stands.
func Reaffirm(repo string, ids []string, a Agent) (Commit, error) {
	c, err := multi(repo, ids, "Reaffirmed", "", a)
	c.Subject = "Reaffirm " + plural(ids)
	return c, err
}

// Review composes an empty commit that accepts each task's work at gate.
func Review(repo string, ids []string, gate string, a Agent) (Commit, error) {
	if err := checkGate(repo, gate); err != nil {
		return Commit{}, err
	}
	c, err := multi(repo, ids, "Reviewed", gate, a)
	c.Subject = "Accept " + plural(ids) + " at " + gate
	return c, err
}

func plural(ids []string) string {
	if len(ids) == 1 {
		return "task " + ids[0]
	}
	return "tasks " + strings.Join(ids, ", ")
}

// Status is the set of fields Record writes. An empty Reason or Note removes
// the line; Gate and State are required.
type Status struct {
	ID, Gate, State, Reason, Note string
	// SelfReview adds "Reviewed: <id> <gate>" for a contributor who is also
	// the reviewer, as an agent is at a junction that states none.
	SelfReview bool
}

// Record composes the commit that writes the task's status file.
func Record(repo string, s Status, a Agent) (Commit, error) {
	if err := checkTask(repo, s.ID); err != nil {
		return Commit{}, err
	}
	if err := checkGate(repo, s.Gate); err != nil {
		return Commit{}, err
	}
	for _, k := range []string{s.State, s.Reason} {
		if k != "" && !keyRe.MatchString(k) {
			return Commit{}, fmt.Errorf("%q is not a key", k)
		}
	}
	if s.State == "" {
		return Commit{}, errors.New("a status needs a state")
	}
	old, _ := os.ReadFile(filepath.Join(repo, statusPath(s.ID)))
	text := string(old)
	text = setField(text, "gate", s.Gate)
	text = setField(text, "state", s.State)
	text = setField(text, "reason", s.Reason)
	text = setField(text, "note", s.Note)
	c := Commit{
		Subject: fmt.Sprintf("Record task %s at the %s gate", s.ID, s.Gate),
		Files:   map[string]string{statusPath(s.ID): text},
	}
	if s.SelfReview {
		c.Trailers = append(c.Trailers, Trailer{"Reviewed", s.ID + " " + s.Gate})
	}
	c.Trailers = append(c.Trailers, a.trailers()...)
	return c, nil
}

var fieldOrder = []string{"gate", "state", "reason", "note"}

// setField replaces, removes or inserts the top-level line "key: value" and
// leaves every other line, comment and blank as it stands. A folded or
// literal block value is replaced whole. An empty value removes the field.
func setField(text, key, value string) string {
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	at, end := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			at, end = i, i+1
			for end < len(lines) && (strings.HasPrefix(lines[end], " ") || strings.HasPrefix(lines[end], "\t")) {
				end++
			}
			break
		}
	}
	newLine := ""
	if value != "" {
		newLine = key + ": " + scalar(value) + "\n"
	}
	switch {
	case at >= 0 && end == at+1 && sameValue(lines[at], key, value):
		// Unchanged: keep the line byte for byte, spacing included.
	case at >= 0:
		lines = splice(lines, at, end, newLine)
	case value == "":
	default:
		// Insert after the nearest earlier field in canonical order, else
		// before the nearest later one, else at the end.
		pos := len(lines)
		k := indexOf(fieldOrder, key)
		found := false
		for j := k - 1; j >= 0 && !found; j-- {
			for i, l := range lines {
				if strings.HasPrefix(l, fieldOrder[j]+":") {
					e := i + 1
					for e < len(lines) && strings.HasPrefix(lines[e], " ") {
						e++
					}
					pos, found = e, true
				}
			}
		}
		if !found {
			for j := k + 1; j < len(fieldOrder) && !found; j++ {
				for i, l := range lines {
					if strings.HasPrefix(l, fieldOrder[j]+":") {
						pos, found = i, true
						break
					}
				}
			}
		}
		if pos > 0 && !strings.HasSuffix(lines[pos-1], "\n") {
			lines[pos-1] += "\n"
		}
		lines = splice(lines, pos, pos, newLine)
	}
	return strings.Join(lines, "")
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func splice(lines []string, from, to int, repl string) []string {
	out := append([]string{}, lines[:from]...)
	if repl != "" {
		out = append(out, repl)
	}
	return append(out, lines[to:]...)
}

var plainRe = regexp.MustCompile(`^[A-Za-z0-9./_~][^"'\[\]{},` + "`" + `]*$`)

// scalar writes v as a plain YAML scalar when that reads back as the same
// string, and as a double-quoted one otherwise.
func scalar(v string) string {
	if plainRe.MatchString(v) && !strings.HasSuffix(v, " ") && !strings.HasSuffix(v, ":") &&
		!strings.Contains(v, ": ") && !strings.Contains(v, " #") && !isReserved(v) {
		return v
	}
	return strconv.Quote(v)
}

func isReserved(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	_, err := strconv.ParseFloat(v, 64)
	return err == nil
}

// Proposal is a new task.
type Proposal struct {
	Title, Description, Assignee string
	Parent                       string
	References                   []string // URLs or paths
}

// Propose composes the commit that adds a task file under a fresh id. The
// task is proposed, not authorised: its authority accepts it with Authorise.
// newID supplies candidate ids; nil draws from crypto/rand.
func Propose(repo string, p Proposal, a Agent, newID func() string) (Commit, string, error) {
	if p.Title == "" || p.Description == "" || p.Assignee == "" {
		return Commit{}, "", errors.New("a task needs a title, description and assignee")
	}
	if err := checkTask(repo, p.Parent); err != nil {
		return Commit{}, "", fmt.Errorf("parent: %w", err)
	}
	if newID == nil {
		newID = randomID
	}
	var id string
	for try := 0; ; try++ {
		if try == 1000 {
			return Commit{}, "", errors.New("no free id")
		}
		id = newID()
		if !exists(repo, taskPath(id)) && !exists(repo, statusPath(id)) {
			break
		}
	}
	order, err := nextOrder(repo, p.Parent)
	if err != nil {
		return Commit{}, "", err
	}
	var b strings.Builder
	b.WriteString("title: " + scalar(p.Title) + "\n")
	b.WriteString("description: >\n")
	for _, l := range wrap(p.Description, 78) {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("assignee: " + p.Assignee + "\n")
	if len(p.References) > 0 {
		b.WriteString("references:\n")
		for _, r := range p.References {
			b.WriteString("  - { url: " + scalar(r) + " }\n")
		}
	}
	fmt.Fprintf(&b, "parent: { id: %q, order: %d }\n", p.Parent, order)
	c := Commit{
		Subject:  "Propose " + strings.ToLower(p.Title[:1]) + p.Title[1:] + " (" + id + ")",
		Trailers: a.trailers(),
		Files:    map[string]string{taskPath(id): b.String()},
	}
	return c, id, nil
}

func exists(repo, rel string) bool {
	_, err := os.Stat(filepath.Join(repo, rel))
	return err == nil
}

func randomID() string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%02x%02x", b[0], b[1])
}

var parentRe = regexp.MustCompile(`(?m)^parent:\s*\{\s*id:\s*"?([0-9a-f]{4})"?\s*,\s*order:\s*(\d+)`)

// nextOrder is one more than the largest order among the parent's children.
func nextOrder(repo, parent string) (int, error) {
	ents, err := os.ReadDir(filepath.Join(repo, ".tableaux", "tasks"))
	if err != nil {
		return 0, err
	}
	max := 0
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(repo, ".tableaux", "tasks", e.Name()))
		if err != nil {
			return 0, err
		}
		if m := parentRe.FindStringSubmatch(string(b)); m != nil && m[1] == parent {
			if n, _ := strconv.Atoi(m[2]); n > max {
				max = n
			}
		}
	}
	return max + 1, nil
}

func wrap(s string, width int) []string {
	var out []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > width {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

// Show returns what Apply will commit: the message, then the change to each
// file as a unified diff against the working tree.
func (c Commit) Show(repo string) (string, error) {
	var b strings.Builder
	b.WriteString("--- commit message ---\n" + c.Message())
	if len(c.Files) == 0 {
		b.WriteString("--- no file changes (empty commit) ---\n")
		return b.String(), nil
	}
	tmp, err := os.MkdirTemp("", "b618-show")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	paths := make([]string, 0, len(c.Files))
	for p := range c.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		old := filepath.Join(repo, p)
		if !exists(repo, p) {
			old = "/dev/null"
		}
		nw := filepath.Join(tmp, "new")
		if err := os.WriteFile(nw, []byte(c.Files[p]), 0o644); err != nil {
			return "", err
		}
		// git diff --no-index exits 1 when the files differ.
		out, _ := exec.Command("git", "diff", "--no-index", "--no-color", "--", old, nw).Output()
		b.WriteString("--- " + p + " ---\n")
		b.WriteString(diffBody(string(out)))
	}
	return b.String(), nil
}

// diffBody drops the diff's header lines and keeps the hunks.
func diffBody(d string) string {
	if i := strings.Index(d, "@@"); i >= 0 {
		return d[i:]
	}
	return "(no change)\n"
}

// Apply writes the files, stages them and commits with the message. env adds
// to the process environment, for the author and committer identity. An
// empty commit needs no file.
func (c Commit) Apply(repo string, env []string) error {
	paths := make([]string, 0, len(c.Files))
	for p, content := range c.Files {
		full := filepath.Join(repo, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
		paths = append(paths, p)
	}
	if len(paths) > 0 {
		if err := git(repo, env, append([]string{"add", "--"}, paths...)...); err != nil {
			return err
		}
	}
	args := []string{"commit", "--cleanup=verbatim", "-m", c.Message()}
	if len(paths) == 0 {
		args = append(args, "--allow-empty")
	}
	return git(repo, env, args...)
}

func git(repo string, env []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %v: %s", args[0], err, out)
	}
	return nil
}

// sameValue reports whether the line "key: ..." already holds value.
func sameValue(line, key, value string) bool {
	v := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
	if u, err := strconv.Unquote(v); err == nil {
		v = u
	}
	return v == value
}
