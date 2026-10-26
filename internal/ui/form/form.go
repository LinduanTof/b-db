// Package form renders a centered modal with a small list of labeled fields.
// It backs the insert-row, import and schema-management flows, which all need
// the same "fill in a few values, preview, confirm" interaction.
package form

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

// Field is one row in the form. A field with Choices becomes a left/right
// picker instead of a text input.
type Field struct {
	Label       string
	Value       string
	Placeholder string
	Choices     []string
	ValueIndex  int
	Mask        bool
}

// SubmittedMsg is emitted when the last field is confirmed with enter.
type SubmittedMsg struct {
	ID      string
	Values  []string
	Choices []int
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	id            string
	title         string
	note          string
	fields        []Field
	inputs        []textinput.Model
	focus         int
	errText       string
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

// Open resets the form with the given fields. Values are copied so callers can
// keep reusing the same slice.
func (m *Model) Open(id, title, note string, fields []Field) {
	m.open = true
	m.id = id
	m.title = title
	m.note = note
	m.errText = ""
	m.focus = 0
	m.fields = append([]Field(nil), fields...)
	m.inputs = make([]textinput.Model, len(m.fields))
	for i, f := range m.fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.CharLimit = 4096
		ti.Placeholder = f.Placeholder
		ti.SetWidth(40)
		ti.SetValue(f.Value)
		ti.CursorEnd()
		if f.Mask {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		m.inputs[i] = ti
	}
	m.focusField(0)
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) SetError(text string) {
	m.errText = text
	m.open = true
}

func (m *Model) focusField(index int) {
	if len(m.fields) == 0 {
		return
	}
	m.focus = min(max(index, 0), len(m.fields)-1)
	for i := range m.inputs {
		if i == m.focus {
			_ = m.inputs[i].Focus()
			m.inputs[i].CursorEnd()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *Model) moveFocus(delta int) {
	m.focusField(m.focus + delta)
}

func (m *Model) submit() (Model, tea.Cmd) {
	values := make([]string, len(m.fields))
	choices := make([]int, len(m.fields))
	for i, f := range m.fields {
		choices[i] = f.ValueIndex
		if len(f.Choices) > 0 {
			values[i] = f.Choices[min(max(f.ValueIndex, 0), len(f.Choices)-1)]
			continue
		}
		values[i] = m.inputs[i].Value()
	}
	id := m.id
	m.open = false
	return *m, func() tea.Msg { return SubmittedMsg{ID: id, Values: values, Choices: choices} }
}

func (m Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.open = false
		return m, nil
	case "tab", "down":
		m.moveFocus(1)
		return m, nil
	case "shift+tab", "up":
		m.moveFocus(-1)
		return m, nil
	case "enter":
		if m.focus >= len(m.fields)-1 {
			return m.submit()
		}
		m.moveFocus(1)
		return m, nil
	}
	if len(m.fields) == 0 {
		return m, nil
	}
	if len(m.fields[m.focus].Choices) > 0 {
		switch msg.String() {
		case "left", "h":
			m.cycleChoice(-1)
		case "right", "l":
			m.cycleChoice(1)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m *Model) cycleChoice(delta int) {
	f := &m.fields[m.focus]
	if len(f.Choices) == 0 {
		return
	}
	f.ValueIndex = (f.ValueIndex + delta + len(f.Choices)) % len(f.Choices)
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	boxW := min(78, m.width-4)
	if boxW < 44 {
		boxW = max(30, m.width-2)
	}
	contentW := max(24, boxW-6)

	labelW := 0
	for _, f := range m.fields {
		labelW = max(labelW, textutil.Width(f.Label))
	}
	labelW = min(labelW, 20)
	inputW := max(12, contentW-labelW-6)
	for i := range m.inputs {
		m.inputs[i].SetWidth(inputW)
	}

	lines := []string{theme.Title.Render(" " + m.title), ""}
	for i, f := range m.fields {
		marker := "  "
		labelStyle := lipgloss.NewStyle()
		if i == m.focus {
			marker = "▸ "
			labelStyle = theme.Header
		}
		label := labelStyle.Render(textutil.FitLine(f.Label, labelW))
		value := ""
		if len(f.Choices) > 0 {
			choice := f.Choices[min(max(f.ValueIndex, 0), len(f.Choices)-1)]
			value = theme.Header.Render("‹ " + choice + " ›")
		} else {
			value = m.inputs[i].View()
		}
		lines = append(lines, " "+marker+label+" "+value)
	}
	if m.note != "" {
		lines = append(lines, "", theme.Muted.Render(" "+textutil.FitLine(m.note, contentW)))
	}
	if m.errText != "" {
		lines = append(lines, theme.Error.Render(" "+textutil.FitLine(m.errText, contentW)))
	}
	hint := m.t.T(i18n.KeyFormHint)
	lines = append(lines, "", theme.Muted.Render(" "+textutil.FitLine(hint, contentW)))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}
