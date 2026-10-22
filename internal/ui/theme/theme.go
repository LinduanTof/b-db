package theme

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

type Palette struct {
	Title       string
	Header      string
	Muted       string
	Error       string
	SelectedFG  string
	SelectedBG  string
	CursorFG    string
	CursorBG    string
	Null        string
	StatusFG    string
	StatusBG    string
	StatusErrFG string
	StatusErrBG string
	MenuFG      string
	MenuBG      string

	SyntaxKeyword  string
	SyntaxString   string
	SyntaxComment  string
	SyntaxNumber   string
	SyntaxFunction string
}

var palettes = map[string]Palette{
	"default": {
		Title: "117", Header: "117", Muted: "244", Error: "203",
		SelectedFG: "255", SelectedBG: "237",
		CursorFG: "255", CursorBG: "25",
		Null:     "240",
		StatusFG: "250", StatusBG: "236",
		StatusErrFG: "224", StatusErrBG: "88",
		MenuFG: "117", MenuBG: "236",
		SyntaxKeyword: "117", SyntaxString: "114", SyntaxComment: "244", SyntaxNumber: "215", SyntaxFunction: "183",
	},
	"dracula": {
		Title: "#bd93f9", Header: "#8be9fd", Muted: "#6272a4", Error: "#ff5555",
		SelectedFG: "#f8f8f2", SelectedBG: "#44475a",
		CursorFG: "#282a36", CursorBG: "#ff79c6",
		Null:     "#6272a4",
		StatusFG: "#f8f8f2", StatusBG: "#44475a",
		StatusErrFG: "#282a36", StatusErrBG: "#ff5555",
		MenuFG: "#8be9fd", MenuBG: "#44475a",
		SyntaxKeyword: "#ff79c6", SyntaxString: "#f1fa8c", SyntaxComment: "#6272a4", SyntaxNumber: "#bd93f9", SyntaxFunction: "#8be9fd",
	},
	"gruvbox": {
		Title: "#b8bb26", Header: "#8ec07c", Muted: "#928374", Error: "#fb4934",
		SelectedFG: "#ebdbb2", SelectedBG: "#3c3836",
		CursorFG: "#282828", CursorBG: "#fabd2f",
		Null:     "#928374",
		StatusFG: "#ebdbb2", StatusBG: "#3c3836",
		StatusErrFG: "#282828", StatusErrBG: "#fb4934",
		MenuFG: "#fabd2f", MenuBG: "#3c3836",
		SyntaxKeyword: "#fb4934", SyntaxString: "#b8bb26", SyntaxComment: "#928374", SyntaxNumber: "#d3869b", SyntaxFunction: "#fabd2f",
	},
	"nord": {
		Title: "#88c0d0", Header: "#81a1c1", Muted: "#616e88", Error: "#bf616a",
		SelectedFG: "#eceff4", SelectedBG: "#434c5e",
		CursorFG: "#2e3440", CursorBG: "#88c0d0",
		Null:     "#616e88",
		StatusFG: "#d8dee9", StatusBG: "#3b4252",
		StatusErrFG: "#eceff4", StatusErrBG: "#bf616a",
		MenuFG: "#88c0d0", MenuBG: "#3b4252",
		SyntaxKeyword: "#81a1c1", SyntaxString: "#a3be8c", SyntaxComment: "#616e88", SyntaxNumber: "#b48ead", SyntaxFunction: "#88c0d0",
	},
	"catppuccin": {
		Title: "#cba6f7", Header: "#89b4fa", Muted: "#6c7086", Error: "#f38ba8",
		SelectedFG: "#cdd6f4", SelectedBG: "#45475a",
		CursorFG: "#1e1e2e", CursorBG: "#89b4fa",
		Null:     "#6c7086",
		StatusFG: "#cdd6f4", StatusBG: "#313244",
		StatusErrFG: "#1e1e2e", StatusErrBG: "#f38ba8",
		MenuFG: "#94e2d5", MenuBG: "#313244",
		SyntaxKeyword: "#cba6f7", SyntaxString: "#a6e3a1", SyntaxComment: "#6c7086", SyntaxNumber: "#fab387", SyntaxFunction: "#89b4fa",
	},
	"solarized-light": {
		Title: "#268bd2", Header: "#2aa198", Muted: "#93a1a1", Error: "#dc322f",
		SelectedFG: "#586e75", SelectedBG: "#eee8d5",
		CursorFG: "#fdf6e3", CursorBG: "#268bd2",
		Null:     "#93a1a1",
		StatusFG: "#586e75", StatusBG: "#eee8d5",
		StatusErrFG: "#fdf6e3", StatusErrBG: "#dc322f",
		MenuFG: "#268bd2", MenuBG: "#eee8d5",
		SyntaxKeyword: "#859900", SyntaxString: "#2aa198", SyntaxComment: "#93a1a1", SyntaxNumber: "#d33682", SyntaxFunction: "#268bd2",
	},
}

var current = "default"

var (
	Title       lipgloss.Style
	Header      lipgloss.Style
	Muted       lipgloss.Style
	Error       lipgloss.Style
	Selected    lipgloss.Style
	CellCursor  lipgloss.Style
	Null        lipgloss.Style
	Status      lipgloss.Style
	StatusError lipgloss.Style
	MenuBar     lipgloss.Style

	SyntaxKeyword  lipgloss.Style
	SyntaxString   lipgloss.Style
	SyntaxComment  lipgloss.Style
	SyntaxNumber   lipgloss.Style
	SyntaxFunction lipgloss.Style
)

func init() {
	Set("default")
}

func Names() []string {
	names := make([]string, 0, len(palettes))
	for name := range palettes {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "default" {
			return true
		}
		if names[j] == "default" {
			return false
		}
		return names[i] < names[j]
	})
	return names
}

func Exists(name string) bool {
	_, ok := palettes[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func Current() string { return current }

func AccentColor() string {
	if p, ok := palettes[current]; ok {
		return p.Title
	}
	return "117"
}

func Set(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	p, ok := palettes[key]
	if !ok {
		key = "default"
		p = palettes[key]
	}

	Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Title))
	Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Header))
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Muted))
	Error = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Error))
	Selected = lipgloss.NewStyle().Background(lipgloss.Color(p.SelectedBG)).Foreground(lipgloss.Color(p.SelectedFG))
	CellCursor = lipgloss.NewStyle().Background(lipgloss.Color(p.CursorBG)).Foreground(lipgloss.Color(p.CursorFG))
	Null = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Null)).Italic(true)
	Status = lipgloss.NewStyle().Foreground(lipgloss.Color(p.StatusFG)).Background(lipgloss.Color(p.StatusBG))
	StatusError = lipgloss.NewStyle().Foreground(lipgloss.Color(p.StatusErrFG)).Background(lipgloss.Color(p.StatusErrBG))
	MenuBar = lipgloss.NewStyle().Foreground(lipgloss.Color(p.MenuFG)).Background(lipgloss.Color(p.MenuBG))

	SyntaxKeyword = lipgloss.NewStyle().Foreground(lipgloss.Color(p.SyntaxKeyword))
	SyntaxString = lipgloss.NewStyle().Foreground(lipgloss.Color(p.SyntaxString))
	SyntaxComment = lipgloss.NewStyle().Foreground(lipgloss.Color(p.SyntaxComment)).Italic(true)
	SyntaxNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(p.SyntaxNumber))
	SyntaxFunction = lipgloss.NewStyle().Foreground(lipgloss.Color(p.SyntaxFunction))

	current = key
	return key
}
