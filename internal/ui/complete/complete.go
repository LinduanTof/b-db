package complete

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ac "github.com/linduan-guan/b-db/internal/complete"
	"github.com/linduan-guan/b-db/internal/i18n"
	"github.com/linduan-guan/b-db/internal/ui/textutil"
	"github.com/linduan-guan/b-db/internal/ui/theme"
)

type AcceptedMsg struct {
	Text      string
	PrefixLen int
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	prefix        string
	candidates    []ac.Candidate
	selected      int
	offset        int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(prefix string, candidates []ac.Candidate) {
	m.open = true
	m.prefix = prefix
	m.candidates = append([]ac.Candidate(nil), candidates...)
	m.selected = 0
	m.offset = 0
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m Model) Prefix() string { return m.prefix }

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
	case "tab", "enter":
		candidate, ok := m.selectedCandidate()
		m.open = false
		if ok {
			prefixLen := len([]rune(m.prefix))
			return *m, func() tea.Msg {
				return AcceptedMsg{Text: candidate.Text, PrefixLen: prefixLen}
			}
		}
	}
	return *m, nil
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(56, m.width-4)
	if boxW < 30 {
		boxW = max(20, m.width-2)
	}
	contentW := max(10, boxW-4)

	maxVisible := max(1, min(8, m.height-6))
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+maxVisible {
		m.offset = m.selected - maxVisible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}

	title := m.t.T(i18n.KeyCompletionTitle)
	if m.prefix != "" {
		title += ": " + m.prefix
	}
	lines := []string{theme.Title.Render(textutil.FitLine(title, contentW))}

	for i := m.offset; i < len(m.candidates) && i < m.offset+maxVisible; i++ {
		c := m.candidates[i]
		line := textutil.FitLine(fmt.Sprintf(" [%s] %s", c.Kind.Label(), c.Text), contentW)
		if i == m.selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	for len(lines) < maxVisible+1 {
		lines = append(lines, "")
	}
	lines = append(lines, theme.Muted.Render(textutil.FitLine(" "+m.t.T(i18n.KeyCompletionHint), contentW)))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(0, 1).
		Width(contentW).
		Render(content)
}

func (m *Model) selectedCandidate() (ac.Candidate, bool) {
	if m.selected < 0 || m.selected >= len(m.candidates) {
		return ac.Candidate{}, false
	}
	return m.candidates[m.selected], true
}

func (m *Model) clamp() {
	if len(m.candidates) == 0 {
		m.selected = 0
		return
	}
	m.selected = min(max(m.selected, 0), len(m.candidates)-1)
}
