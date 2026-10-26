package confirm

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

type ConfirmedMsg struct{}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	title         string
	message       string
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(title, message string) {
	m.open = true
	m.title = title
	m.message = message
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return *m, nil
	}
	switch msg.String() {
	case "y", "enter":
		m.open = false
		return *m, func() tea.Msg { return ConfirmedMsg{} }
	case "n", "esc":
		m.open = false
	}
	return *m, nil
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(78, m.width-4)
	if boxW < 40 {
		boxW = max(30, m.width-2)
	}
	contentW := max(20, boxW-6)

	lines := []string{theme.Title.Render(" " + m.title), ""}
	maxLines := max(1, m.height-10)
	messageLines := strings.Split(m.message, "\n")
	for i, line := range messageLines {
		if i >= maxLines {
			lines = append(lines, theme.Muted.Render(" ..."))
			break
		}
		lines = append(lines, " "+textutil.Truncate(line, contentW-2))
	}
	lines = append(lines, "", theme.Muted.Render(" "+m.t.T(i18n.KeyConfirmHint)))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}
