package command

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

type Definition struct {
	Name    string
	Args    string
	DescKey string
}

var defs = []Definition{
	{Name: "connect", Args: "<name>", DescKey: i18n.KeyCmdConnect},
	{Name: "disconnect", DescKey: i18n.KeyCmdDisconnect},
	{Name: "describe", DescKey: i18n.KeyCmdDescribe},
	{Name: "export", Args: "csv|json|md", DescKey: i18n.KeyCmdExport},
	{Name: "focus", Args: "sidebar|editor|results", DescKey: i18n.KeyCmdFocus},
	{Name: "lang", Args: "en|zh", DescKey: i18n.KeyCmdLang},
	{Name: "run", Args: "<path>", DescKey: i18n.KeyCmdRun},
	{Name: "help", DescKey: i18n.KeyCmdHelp},
	{Name: "reload", DescKey: i18n.KeyCmdReload},
	{Name: "quit", DescKey: i18n.KeyCmdQuit},
}

func Definitions() []Definition {
	out := make([]Definition, len(defs))
	copy(out, defs)
	return out
}

type SelectedMsg struct {
	Name string
	Args []string
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	input         string
	selected      int
	offset        int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Open() {
	m.open = true
	m.input = ""
	m.selected = 0
	m.offset = 0
}

func (m *Model) Close() { m.open = false }

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.open = false
	case "up", "ctrl+p":
		m.selected--
		m.clamp()
	case "down", "ctrl+n":
		m.selected++
		m.clamp()
	case "tab":
		if d := m.selectedDef(); d != nil {
			m.input = d.Name + " "
		}
		m.selected = 0
	case "enter":
		d := m.selectedDef()
		if d == nil {
			return *m, nil
		}
		if strings.TrimSpace(m.input) == "" {
			m.input = d.Name + " "
			return *m, nil
		}
		args := parseArgs(m.input, d.Name)
		name := d.Name
		m.open = false
		return *m, func() tea.Msg { return SelectedMsg{Name: name, Args: args} }
	case "backspace", "ctrl+h":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
			m.selected = 0
		}
	default:
		if msg.Text != "" {
			m.input += msg.Text
			m.selected = 0
		}
	}
	return *m, nil
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(78, m.width-2)
	if boxW < 30 {
		boxW = m.width
	}
	contentW := max(10, boxW-4)

	matches := m.filtered()
	maxRows := max(1, min(len(matches), m.height-8))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+maxRows {
		m.offset = m.selected - maxRows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}

	lines := []string{
		theme.Title.Render(textutil.FitLine(m.t.T(i18n.KeyCommandTitle), contentW)),
		" " + theme.Header.Render(":") + " " + m.input + theme.CellCursor.Render(" "),
		"",
	}

	nameWidth := 24
	for i := m.offset; i < len(matches) && i < m.offset+maxRows; i++ {
		d := matches[i]
		name := d.Name
		if d.Args != "" {
			name += " " + d.Args
		}
		line := " " + name
		if w := textutil.Width(line); w < nameWidth {
			line += strings.Repeat(" ", nameWidth-w)
		}
		descW := contentW - nameWidth
		desc := ""
		if descW > 3 {
			desc = textutil.Truncate(m.t.T(d.DescKey), descW-2)
		}
		line += "  " + desc
		if i == m.selected {
			lines = append(lines, theme.Selected.Render(textutil.FitLine(line, contentW)))
		} else {
			lines = append(lines, textutil.FitLine(line, contentW))
		}
	}
	if len(matches) == 0 {
		lines = append(lines, theme.Muted.Render(textutil.FitLine(" no matching command", contentW)))
	}
	for len(lines) < maxRows+3 {
		lines = append(lines, "")
	}
	lines = append(lines, theme.Muted.Render(textutil.FitLine(" "+m.t.T(i18n.KeyCommandHint), contentW)))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("117")).
		Padding(0, 1).
		Width(contentW).
		Render(content)
}

func (m *Model) selectedDef() *Definition {
	matches := m.filtered()
	if len(matches) == 0 {
		return nil
	}
	idx := min(max(m.selected, 0), len(matches)-1)
	return &matches[idx]
}

func (m *Model) clamp() {
	n := len(m.filtered())
	if n == 0 {
		m.selected = 0
		return
	}
	m.selected = min(max(m.selected, 0), n-1)
}

func (m *Model) filtered() []Definition {
	q := strings.TrimSpace(m.input)
	if i := strings.IndexAny(q, " \t"); i >= 0 {
		q = q[:i]
	}
	q = strings.ToLower(q)
	if q == "" {
		return defs
	}
	out := make([]Definition, 0, len(defs))
	for _, d := range defs {
		if strings.Contains(d.Name, q) {
			out = append(out, d)
		}
	}
	return out
}

func parseArgs(input, name string) []string {
	s := strings.TrimSpace(input)
	if strings.HasPrefix(s, name) {
		s = strings.TrimSpace(s[len(name):])
	} else {
		s = ""
	}
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}
