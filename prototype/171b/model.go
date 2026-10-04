package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChangedMsg reports that the watched source settled on a new fingerprint.
type ChangedMsg struct{ FP Fingerprint }

// LoadedMsg carries the result of one reload.
type LoadedMsg struct {
	Seq  int
	View View
	Err  error
}

// Model is the one-screen status.
type Model struct {
	ctx    context.Context
	src    Source
	loader Loader
	watch  *Watcher
	fp     Fingerprint // the last fingerprint the watcher armed against
	seq    int         // the newest load requested

	view       View
	loaded     bool
	err        error
	reloads    int // successful reloads after the first load
	lastReload time.Time
	width      int
	now        func() time.Time
}

// NewModel returns a model whose watcher starts from the baseline fingerprint.
func NewModel(ctx context.Context, src Source, l Loader, w *Watcher, base Fingerprint) Model {
	return Model{ctx: ctx, src: src, loader: l, watch: w, fp: base, seq: 1, now: time.Now}
}

func (m Model) loadCmd(seq int) tea.Cmd {
	return func() tea.Msg {
		v, err := m.loader.Load(m.ctx, m.src)
		return LoadedMsg{Seq: seq, View: v, Err: err}
	}
}

// watchCmd waits for the next change and returns it as one message. Update
// re-arms it after each ChangedMsg, so there is one waiter at a time. It runs
// in the goroutine Bubble Tea gives every Cmd, so it never blocks the loop.
func (m Model) watchCmd(prev Fingerprint) tea.Cmd {
	if !m.src.Watch || m.watch == nil {
		return nil
	}
	return func() tea.Msg {
		fp, err := m.watch.Next(m.ctx, prev)
		if err != nil {
			return nil // cancelled: the program is ending
		}
		return ChangedMsg{FP: fp}
	}
}

// Init starts the first load and the first wait.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadCmd(1), m.watchCmd(m.fp))
}

// Update handles keys, size, changes and loads.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s := msg.String(); s == "q" || s == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case ChangedMsg:
		m.fp = msg.FP
		m.seq++
		return m, tea.Batch(m.loadCmd(m.seq), m.watchCmd(msg.FP))
	case LoadedMsg:
		if msg.Seq < m.seq {
			return m, nil // a newer load is under way
		}
		if msg.Err != nil {
			m.err = msg.Err // keep the last good view
			return m, nil
		}
		if m.loaded {
			m.reloads++
		}
		m.view, m.loaded, m.err = msg.View, true, nil
		m.lastReload = m.now()
	}
	return m, nil
}

func (m Model) rows(chrome bool) string {
	label := lipgloss.NewStyle().Bold(true).Width(9)
	row := func(k, v string) string {
		line := label.Render(k) + v
		if m.width > 0 {
			line = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
		}
		return line
	}
	var b strings.Builder
	if chrome {
		b.WriteString(row("tablotui", "171b live refresh") + "\n\n")
	}
	if m.loaded {
		b.WriteString(row("commit", m.view.Commit) + "\n")
		b.WriteString(row("digest", fmt.Sprintf("%s (%d files)", m.view.Digest, m.view.Files)) + "\n")
	} else {
		b.WriteString(row("commit", "loading") + "\n")
	}
	b.WriteString(row("ref", m.src.Ref) + "\n")
	b.WriteString(row("mode", m.src.Mode()) + "\n")
	if chrome {
		b.WriteString(row("reloads", fmt.Sprint(m.reloads)) + "\n")
		last := "never"
		if !m.lastReload.IsZero() {
			last = m.lastReload.Format("15:04:05")
		}
		b.WriteString(row("loaded", last) + "\n")
		if m.err != nil {
			b.WriteString(row("error", m.err.Error()+" (showing the last good view)") + "\n")
		}
		b.WriteString("\nq quits\n")
	}
	return b.String()
}

// Data renders the project data alone: commit, digest and file count, ref and
// mode. The title, reload counter, load time, error line and key hint are
// chrome.
func (m Model) Data() string { return m.rows(false) }

// View renders the status with its chrome.
func (m Model) View() string { return m.rows(true) }
