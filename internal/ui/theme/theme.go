package theme

import "charm.land/lipgloss/v2"

var (
	Title       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	Header      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	Muted       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	Error       = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	Selected    = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("255"))
	CellCursor  = lipgloss.NewStyle().Background(lipgloss.Color("25")).Foreground(lipgloss.Color("255"))
	Null        = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	Status      = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236"))
	StatusError = lipgloss.NewStyle().Foreground(lipgloss.Color("224")).Background(lipgloss.Color("88"))
	MenuBar     = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Background(lipgloss.Color("236"))
)
