package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

type Kind int

const (
	KindHeader Kind = iota
	KindAction
	KindChoice
)

type Item struct {
	ID       string
	LabelKey string
	Kind     Kind
	Choices  []string
	Value    int
}

type ActionMsg struct {
	ID string
}

type ChangedMsg struct {
	ID    string
	Value int
}

type Model struct {
	t *i18n.Translator

	width, height int
	open          bool
	items         []Item
	selected      int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(items []Item) {
	m.open = true
	m.items = append([]Item(nil), items...)
	m.selected = 0
	if len(m.items) > 0 {
		m.normalizeSelection(1)
	}
}

func (m *Model) Close() { m.open = false }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Update(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.open {
		return *m, nil
	}
	switch msg.String() {
	case "esc":
		m.open = false
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		return *m, m.adjust(-1)
	case "right", "l":
		return *m, m.adjust(1)
	case "enter":
		item := m.selectedItem()
		if item == nil {
			return *m, nil
		}
		if item.Kind == KindChoice {
			return *m, m.adjust(1)
		}
		m.open = false
		id := item.ID
		return *m, func() tea.Msg { return ActionMsg{ID: id} }
	}
	return *m, nil
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}

	boxW := min(66, m.width-4)
	if boxW < 40 {
		boxW = max(30, m.width-2)
	}
	contentW := max(24, boxW-6)

	title := m.t.T(i18n.KeyMenuTitle)
	titlePad := max(0, (contentW-textutil.Width(title))/2)
	lines := []string{
		strings.Repeat(" ", titlePad) + theme.Title.Render(title),
		"",
	}

	const labelW = 18
	for i, item := range m.items {
		switch item.Kind {
		case KindHeader:
			if item.LabelKey == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, theme.Header.Render(" "+m.t.T(item.LabelKey)))
			}
			continue
		}

		selected := i == m.selected
		marker := " "
		labelStyle := lipgloss.NewStyle()
		if selected {
			marker = "▸"
			labelStyle = theme.Header
		}

		line := " " + marker + " " + labelStyle.Render(pad(m.t.T(item.LabelKey), labelW))
		if item.Kind == KindChoice && len(item.Choices) > 0 {
			value := item.Choices[min(max(item.Value, 0), len(item.Choices)-1)]
			valueStyle := lipgloss.NewStyle()
			if selected {
				valueStyle = theme.Header
			}
			line += valueStyle.Render("◀ " + value + " ▶")
		}
		lines = append(lines, textutil.FitLine(line, contentW))
	}

	lines = append(lines, "",
		theme.Muted.Render(textutil.FitLine(center(m.t.T(i18n.KeyMenuHint), contentW), contentW)),
	)

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.AccentColor())).
		Padding(1, 2).
		Width(contentW).
		Render(content)
}

func (m *Model) selectedItem() *Item {
	if m.selected < 0 || m.selected >= len(m.items) {
		return nil
	}
	return &m.items[m.selected]
}

func (m *Model) move(delta int) {
	if len(m.items) == 0 {
		return
	}
	for step := 0; step < len(m.items); step++ {
		m.selected = (m.selected + delta + len(m.items)) % len(m.items)
		if m.items[m.selected].Kind != KindHeader {
			return
		}
	}
}

func (m *Model) normalizeSelection(delta int) {
	for step := 0; step < len(m.items); step++ {
		if m.items[m.selected].Kind != KindHeader {
			return
		}
		m.selected = (m.selected + delta + len(m.items)) % len(m.items)
	}
}

func (m *Model) adjust(delta int) tea.Cmd {
	item := m.selectedItem()
	if item == nil || item.Kind != KindChoice || len(item.Choices) == 0 {
		return nil
	}
	item.Value = (item.Value + delta + len(item.Choices)) % len(item.Choices)
	id, value := item.ID, item.Value
	return func() tea.Msg { return ChangedMsg{ID: id, Value: value} }
}

func pad(s string, width int) string {
	if d := width - textutil.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func center(s string, width int) string {
	if w := textutil.Width(s); w < width {
		return strings.Repeat(" ", (width-w)/2) + s
	}
	return s
}
