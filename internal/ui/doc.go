// Package ui holds the application frame of the tablotui program: the root
// Bubble Tea model, the Pane, Mode and Command a task plugs into it, the
// messages that carry view data and requests between them, the terminal-aware
// Measure and the Styles. It imports neither the grid nor a data source; the
// command hands the grid to the frame as its first pane.
package ui
