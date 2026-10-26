package pathinput

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	input         textinput.Model
	errText       string
}

type SubmittedMsg struct {
	Value string
}

func New(t *i18n.Translator) Model {
	m := Model{t: t}
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.CharLimit = 512
	return m
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(value string) {
	m.open = true
	m.errText = ""
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *Model) Close() {
	m.open = false
	m.input.Blur()
}

// Paste forwards pasted text to the path input.
func (m *Model) Paste(msg tea.PasteMsg) tea.Cmd {
	if !m.open {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m Model) IsOpen() bool { return m.open }

func (m *Model) SetError(text string) { m.errText = text }

func (m Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.Close()
		return m, nil
	case "enter":
		m.open = false
		m.input.Blur()
		value := strings.TrimSpace(m.input.Value())
		return m, func() tea.Msg { return SubmittedMsg{Value: value} }
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(72, m.width-4)
	if boxW < 40 {
		boxW = max(30, m.width-2)
	}
	contentW := max(24, boxW-6)
	m.input.SetWidth(max(10, contentW-2))

	title := m.t.T(i18n.KeyExportDirTitle)
	titlePad := max(0, (contentW-textutil.Width(title))/2)
	lines := []string{
		strings.Repeat(" ", titlePad) + theme.Title.Render(title),
		"",
		" " + m.input.View(),
		"",
		theme.Muted.Render(textutil.FitLine(" "+m.t.T(i18n.KeyExportDirNote), contentW)),
	}
	if m.errText != "" {
		lines = append(lines, theme.Error.Render(textutil.FitLine(" "+m.errText, contentW)))
	}
	hint := m.t.T(i18n.KeyExportDirHint)
	hintPad := max(0, (contentW-textutil.Width(hint))/2)
	lines = append(lines, "",
		theme.Muted.Render(textutil.FitLine(strings.Repeat(" ", hintPad)+hint, contentW)),
	)

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}
