package ui

import "charm.land/lipgloss/v2"

// Styles holds every style a pane uses. The zero value draws plain text.
type Styles struct{ Title, Header, Parent, Muted, Cursor, Acts, Error lipgloss.Style }

// DefaultStyles returns the styles of the program. It uses the terminal's 16
// colours alone, so the terminal's theme decides light and dark, and it
// colours no symbol. No fact rests on colour: a parent has its marker, an
// acting cell its brackets and an error its prefix.
func DefaultStyles() Styles {
	return Styles{
		Title:  lipgloss.NewStyle().Bold(true),
		Header: lipgloss.NewStyle().Bold(true),
		Parent: lipgloss.NewStyle().Bold(true),
		Muted:  lipgloss.NewStyle().Faint(true),
		Cursor: lipgloss.NewStyle().Reverse(true),
		Acts:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")),
		Error:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")),
	}
}
