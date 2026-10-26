package textview

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	title         string
	text          string
	scroll        int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(title, text string) {
	m.open = true
	m.title = title
	m.text = text
	m.scroll = 0
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Scroll(delta int) {
	m.scroll += delta * 3
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.open = false
	case "up", "k":
		m.scroll--
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll -= m.pageSize()
	case "pgdown":
		m.scroll += m.pageSize()
	case "g":
		m.scroll = 0
	case "G":
		m.scroll = max(0, len(m.lines())-m.pageSize())
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	return *m, nil
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	lines := m.lines()
	size := m.pageSize()
	maxScroll := max(0, len(lines)-size)
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	end := min(len(lines), m.scroll+size)

	out := make([]string, 0, m.height)
	out = append(out, theme.Title.Render(" "+m.title), "")
	for _, line := range lines[m.scroll:end] {
		out = append(out, textutil.FitLine(" "+line, m.width))
	}
	for len(out) < m.height-1 {
		out = append(out, "")
	}
	out = append(out, theme.Muted.Render(" "+m.t.T(i18n.KeyTextViewHint)))
	return textutil.FitLines(strings.Join(out, "\n"), m.width, m.height)
}

func (m *Model) lines() []string {
	if m.text == "" {
		return []string{""}
	}
	return strings.Split(m.text, "\n")
}

func (m *Model) pageSize() int {
	return max(1, m.height-3)
}
