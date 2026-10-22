package editor

import (
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/i18n"
)

const maxHistory = 200

type Model struct {
	t       *i18n.Translator
	ta      textarea.Model
	history []string
	histPos int
}

func New(t *i18n.Translator) Model {
	ta := textarea.New()
	ta.Prompt = "› "
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	return Model{t: t, ta: ta}
}

func (m *Model) SetSize(width, height int) {
	m.ta.SetWidth(max(width, 1))
	m.ta.SetHeight(max(height, 1))
}

func (m *Model) Focus() tea.Cmd { return m.ta.Focus() }

func (m *Model) Blur() { m.ta.Blur() }

func (m Model) Value() string { return m.ta.Value() }

func (m Model) WordBeforeCursor() (string, int) {
	line := m.ta.Line()
	column := m.ta.Column()
	lines := strings.Split(m.ta.Value(), "\n")
	if line < 0 || line >= len(lines) {
		return "", 0
	}
	runes := []rune(lines[line])
	if column > len(runes) {
		column = len(runes)
	}
	start := column
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	return string(runes[start:column]), column - start
}

func (m *Model) ReplaceBeforeCursor(prefixLen int, text string) {
	for i := 0; i < prefixLen; i++ {
		m.ta, _ = m.ta.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	m.ta.InsertString(text)
}

func isWordRune(r rune) bool {
	return r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (m *Model) SetValue(s string) {
	m.ta.SetValue(s)
	m.ta.CursorEnd()
}

func (m Model) View() string { return m.ta.View() }

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return *m, cmd
}

func (m *Model) AddHistory(stmt string) {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != stmt {
		m.history = append(m.history, stmt)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}
	}
	m.histPos = len(m.history)
}

func (m *Model) SetHistory(entries []string) {
	m.history = append([]string(nil), entries...)
	if len(m.history) > maxHistory {
		m.history = m.history[len(m.history)-maxHistory:]
	}
	m.histPos = len(m.history)
}

func (m *Model) BrowseHistory(previous bool) {
	if len(m.history) == 0 {
		return
	}
	if previous {
		if m.histPos > 0 {
			m.histPos--
		}
	} else {
		if m.histPos < len(m.history) {
			m.histPos++
		}
		if m.histPos == len(m.history) {
			m.SetValue("")
			return
		}
	}
	m.SetValue(m.history[m.histPos])
}
