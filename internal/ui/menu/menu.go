package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
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
	title         string
	items         []Item
	selected      int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(title string, items []Item) {
	m.open = true
	m.title = title
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
		return m.activate(m.selected)
	}
	return *m, nil
}

func (m *Model) menuTitle() string {
	if m.title != "" {
		return m.title
	}
	return m.t.T(i18n.KeyMenuTitle)
}

func (m *Model) innerWidth() int {
	natural := 0
	for _, item := range m.items {
		switch item.Kind {
		case KindHeader:
			natural = max(natural, textutil.Width(m.t.T(item.LabelKey))+3)
		case KindChoice:
			value := ""
			if len(item.Choices) > 0 {
				value = item.Choices[min(max(item.Value, 0), len(item.Choices)-1)]
			}
			natural = max(natural, textutil.Width(m.t.T(item.LabelKey))+textutil.Width(value)+11)
		default:
			natural = max(natural, textutil.Width(m.t.T(item.LabelKey))+5)
		}
	}
	natural = max(natural, min(textutil.Width(m.menuTitle()), 40)+6)
	natural += 2
	limit := min(64, m.width-4)
	if limit < 32 {
		limit = max(24, m.width-2)
	}
	return min(max(natural, 34), limit)
}

func (m *Model) bodyLines(inner int) ([]string, []int) {
	width := max(1, inner-2)
	lines := []string{""}
	itemLines := make([]int, len(m.items))
	for i, item := range m.items {
		if item.Kind == KindHeader && item.LabelKey != "" && len(lines) > 1 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		itemLines[i] = len(lines)
		if item.Kind == KindHeader {
			lines = append(lines, m.renderSection(item, width))
			continue
		}
		lines = append(lines, m.renderItem(item, i == m.selected, width))
	}
	lines = append(lines, "")
	return lines, itemLines
}

func (m *Model) renderSection(item Item, width int) string {
	if item.LabelKey == "" {
		return ""
	}
	label := strings.ToUpper(m.t.T(item.LabelKey))
	return theme.Header.Render(textutil.FitLine(" ▎"+label, width))
}

func (m *Model) renderItem(item Item, selected bool, width int) string {
	label := m.t.T(item.LabelKey)
	if item.Kind == KindChoice && len(item.Choices) > 0 {
		value := item.Choices[min(max(item.Value, 0), len(item.Choices)-1)]
		controls := "‹ " + value + " ›"
		gap := width - 3 - textutil.Width(label) - textutil.Width(controls)
		if gap < 1 {
			controls = textutil.Truncate(controls, max(4, width-5-textutil.Width(label)))
			gap = max(1, width-3-textutil.Width(label)-textutil.Width(controls))
		}
		plain := "  " + label + strings.Repeat(" ", gap) + controls + " "
		if selected {
			return theme.CellCursor.Render(textutil.FitLine(plain, width))
		}
		return textutil.FitLine("  "+label+strings.Repeat(" ", gap)+theme.Header.Render(controls), width)
	}
	plain := "   " + label
	if selected {
		plain = " ▸ " + label
		return theme.CellCursor.Render(textutil.FitLine(plain, width))
	}
	return textutil.FitLine(plain, width)
}

func (m *Model) View() string {
	if !m.open || m.width <= 0 || m.height <= 0 {
		return ""
	}
	width := m.innerWidth()
	body, _ := m.bodyLines(width)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentColor()))
	title := textutil.Truncate(m.menuTitle(), max(1, width-4))
	repeat := max(0, width-textutil.Width(title)-3)
	lines := []string{
		accent.Render("┏━ ") + theme.Title.Render(title) + accent.Render(" "+strings.Repeat("━", repeat)+"┓"),
	}
	for _, line := range body {
		lines = append(lines, accent.Render("┃")+" "+textutil.FitLine(line, max(1, width-2))+" "+accent.Render("┃"))
	}
	hint := textutil.Truncate(m.t.T(i18n.KeyMenuHintCompact), max(1, width-4))
	hint = textutil.FitLine(center(hint, max(1, width-2)), max(1, width-2))
	lines = append(lines,
		accent.Render("┣"+strings.Repeat("━", width)+"┫"),
		accent.Render("┃")+" "+theme.Footer.Render(hint)+" "+accent.Render("┃"),
		accent.Render("┗"+strings.Repeat("━", width)+"┛"),
	)
	return strings.Join(lines, "\n")
}

// HandleClick selects and activates the item under (x, y), or closes the menu
// when the click lands outside it.
func (m *Model) HandleClick(x, y, screenW, screenH int) (Model, tea.Cmd) {
	if !m.open {
		return *m, nil
	}
	view := m.View()
	if view == "" {
		return *m, nil
	}
	lines := strings.Split(view, "\n")
	modalW := 0
	for _, line := range lines {
		modalW = max(modalW, ansi.StringWidth(line))
	}
	x0 := max(0, (screenW-modalW)/2)
	y0 := max(0, (screenH-len(lines))/2)
	if x < x0 || x >= x0+modalW || y < y0 || y >= y0+len(lines) {
		m.open = false
		return *m, nil
	}
	_, itemLines := m.bodyLines(m.innerWidth())
	row := y - y0 - 1 // top border
	for i, line := range itemLines {
		if line == row {
			return m.click(i)
		}
	}
	return *m, nil
}

func (m *Model) click(index int) (Model, tea.Cmd) {
	if index < 0 || index >= len(m.items) {
		return *m, nil
	}
	if m.items[index].Kind == KindHeader {
		return *m, nil
	}
	if m.selected != index {
		m.selected = index
		return *m, nil
	}
	return m.activate(index)
}

// HandleWheel moves the selection with the mouse wheel.
func (m *Model) HandleWheel(delta int) {
	if !m.open || delta == 0 {
		return
	}
	if delta < 0 {
		m.move(-1)
		return
	}
	m.move(1)
}

func (m *Model) activate(index int) (Model, tea.Cmd) {
	if index < 0 || index >= len(m.items) {
		return *m, nil
	}
	item := &m.items[index]
	if item.Kind == KindHeader {
		return *m, nil
	}
	m.selected = index
	if item.Kind == KindChoice {
		return *m, m.adjust(1)
	}
	m.open = false
	id := item.ID
	return *m, func() tea.Msg { return ActionMsg{ID: id} }
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

func center(s string, width int) string {
	if w := textutil.Width(s); w < width {
		return strings.Repeat(" ", (width-w)/2) + s
	}
	return s
}
