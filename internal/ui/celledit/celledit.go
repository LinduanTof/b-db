package celledit

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type SubmittedMsg struct {
	Value string
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	title         string
	input         textinput.Model
}

func New(t *i18n.Translator) Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 0
	input.SetWidth(50)
	return Model{t: t, input: input}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(title, value string) {
	m.open = true
	m.title = title
	m.input.SetValue(value)
	m.input.CursorEnd()
	_ = m.input.Focus()
}

func (m *Model) Close() { m.open = false }

// Paste forwards pasted text to the value input.
func (m *Model) Paste(msg tea.PasteMsg) tea.Cmd {
	if !m.open {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) SetValue(value string) {
	m.input.SetValue(value)
	m.input.CursorEnd()
}

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return *m, nil
	}
	switch msg.String() {
	case "esc":
		m.open = false
		return *m, nil
	case "enter":
		value := m.input.Value()
		m.open = false
		return *m, func() tea.Msg { return SubmittedMsg{Value: value} }
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return *m, cmd
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(72, m.width-4)
	if boxW < 40 {
		boxW = max(30, m.width-2)
	}
	contentW := max(20, boxW-6)
	m.input.SetWidth(max(10, contentW-2))

	lines := []string{
		theme.Title.Render(" " + m.title),
		"",
		" " + m.input.View(),
		"",
		theme.Muted.Render(" " + m.t.T(i18n.KeyCellEditHint)),
	}
	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}
