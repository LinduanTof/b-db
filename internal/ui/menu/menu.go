package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
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
	// Label overrides LabelKey for items whose text is not translated, such
	// as the name of a column or index.
	Label   string
	Kind    Kind
	Choices []string
	Value   int
	Tab     int
}

func (m *Model) label(item Item) string {
	if item.Label != "" {
		return item.Label
	}
	return m.t.T(item.LabelKey)
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
	tabs          []string
	tab           int
	tabLine       int
	tabRanges     [][2]int
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) Open(title string, items []Item) {
	m.openWith(title, nil, items)
}

// OpenTabs opens a menu whose items are grouped into tabs. Items select their
// tab with Item.Tab.
func (m *Model) OpenTabs(title string, tabs []string, items []Item) {
	m.openWith(title, tabs, items)
}

func (m *Model) openWith(title string, tabs []string, items []Item) {
	m.open = true
	m.title = title
	m.items = append([]Item(nil), items...)
	m.tabs = append([]string(nil), tabs...)
	m.tab = 0
	m.selected = 0
	m.tabLine = -1
	m.tabRanges = nil
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
	case "tab":
		m.switchTab(1)
	case "shift+tab":
		m.switchTab(-1)
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
			natural = max(natural, textutil.Width(m.label(item))+3)
		case KindChoice:
			value := ""
			if len(item.Choices) > 0 {
				value = item.Choices[min(max(item.Value, 0), len(item.Choices)-1)]
			}
			natural = max(natural, textutil.Width(m.label(item))+textutil.Width(value)+11)
		default:
			natural = max(natural, textutil.Width(m.label(item))+5)
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
	m.tabLine = -1
	m.tabRanges = nil
	if len(m.tabs) > 1 {
		m.tabLine = len(lines)
		tabLine, ranges := m.renderTabs(width)
		m.tabRanges = ranges
		lines = append(lines, tabLine, theme.Muted.Render(strings.Repeat("─", width)))
	}
	itemLines := make([]int, len(m.items))
	for i := range itemLines {
		itemLines[i] = -1
	}
	for i, item := range m.items {
		if len(m.tabs) > 1 && item.Tab != m.tab {
			continue
		}
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

func (m *Model) renderTabs(width int) (string, [][2]int) {
	var b strings.Builder
	b.WriteString(" ")
	x := 1
	ranges := make([][2]int, len(m.tabs))
	for i, label := range m.tabs {
		block := " " + label + " "
		style := theme.Muted
		if i == m.tab {
			style = theme.CellCursor
		}
		ranges[i] = [2]int{x, x + textutil.Width(block)}
		b.WriteString(style.Render(block))
		x += textutil.Width(block) + 1
		b.WriteString(" ")
	}
	return textutil.FitLine(textutil.Truncate(b.String(), width), width), ranges
}

func (m *Model) renderSection(item Item, width int) string {
	if item.LabelKey == "" && item.Label == "" {
		return ""
	}
	label := strings.ToUpper(m.label(item))
	return theme.Header.Render(textutil.FitLine(" ▎"+label, width))
}

func (m *Model) renderItem(item Item, selected bool, width int) string {
	label := m.label(item)
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
	if row == m.tabLine && len(m.tabs) > 1 {
		contentX := x - x0 - 2 // border + left padding
		for i, span := range m.tabRanges {
			if contentX >= span[0] && contentX < span[1] {
				m.selectTab(i)
				return *m, nil
			}
		}
		return *m, nil
	}
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
	item := m.items[index]
	if item.Kind == KindHeader || item.Tab != m.tab {
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
		item := m.items[m.selected]
		if item.Kind != KindHeader && item.Tab == m.tab {
			return
		}
	}
}

func (m *Model) normalizeSelection(delta int) {
	for step := 0; step < len(m.items); step++ {
		item := m.items[m.selected]
		if item.Kind != KindHeader && item.Tab == m.tab {
			return
		}
		m.selected = (m.selected + delta + len(m.items)) % len(m.items)
	}
}

func (m *Model) switchTab(delta int) {
	if len(m.tabs) <= 1 {
		return
	}
	m.selectTab((m.tab + delta + len(m.tabs)) % len(m.tabs))
}

func (m *Model) selectTab(index int) {
	if index < 0 || index >= len(m.tabs) {
		return
	}
	m.tab = index
	for i, item := range m.items {
		if item.Tab == m.tab && item.Kind != KindHeader {
			m.selected = i
			return
		}
	}
	m.selected = 0
}

func (m *Model) adjust(delta int) tea.Cmd {
	item := m.selectedItem()
	if item == nil || item.Kind != KindChoice || item.Tab != m.tab || len(item.Choices) == 0 {
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
