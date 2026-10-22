package results

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/linduan/bdb/internal/db"
	"github.com/linduan/bdb/internal/i18n"
	"github.com/linduan/bdb/internal/ui/textutil"
	"github.com/linduan/bdb/internal/ui/theme"
)

const (
	rowNumWidth = 5
	minColWidth = 4
	maxColWidth = 40
	nullText    = "NULL"
)

type Model struct {
	t *i18n.Translator

	width, height int
	rs            *db.ResultSet
	cells         [][]string
	widths        []int
	affected      *int64
	elapsed       time.Duration
	errText       string

	cursorRow, cursorCol int
	scrollRow, scrollCol int

	order   []int
	sortCol int
	sortAsc bool
}

func New(t *i18n.Translator) Model {
	return Model{t: t}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
}

func (m *Model) SetResult(rs *db.ResultSet) {
	m.rs = rs
	m.errText = ""
	m.affected = nil
	m.elapsed = rs.Elapsed
	m.cursorRow, m.cursorCol = 0, 0
	m.scrollRow, m.scrollCol = 0, 0
	m.sortCol = -1
	m.sortAsc = true
	m.order = make([]int, len(rs.Rows))
	for i := range m.order {
		m.order[i] = i
	}
	m.cells = make([][]string, len(rs.Rows))
	for i, row := range rs.Rows {
		converted := make([]string, len(rs.Columns))
		for j := range rs.Columns {
			if j < len(row) {
				converted[j] = displayValue(row[j])
			}
		}
		m.cells[i] = converted
	}
	m.computeWidths()
}

func (m *Model) SetExec(affected int64, elapsed time.Duration) {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.errText = ""
	m.affected = &affected
	m.elapsed = elapsed
}

func (m *Model) SetError(text string) {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.affected = nil
	m.errText = text
}

func (m *Model) Result() *db.ResultSet { return m.rs }

func (m *Model) Clear() {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.affected = nil
	m.errText = ""
	m.cursorRow, m.cursorCol = 0, 0
	m.scrollRow, m.scrollCol = 0, 0
	m.order = nil
	m.sortCol = -1
	m.sortAsc = true
}

func (m *Model) computeWidths() {
	if m.rs == nil {
		return
	}
	m.widths = make([]int, len(m.rs.Columns))
	for i, col := range m.rs.Columns {
		m.widths[i] = min(max(runewidth.StringWidth(col.Name), minColWidth), maxColWidth)
	}
	sample := min(len(m.cells), 200)
	for r := 0; r < sample; r++ {
		for c, text := range m.cells[r] {
			if c >= len(m.widths) {
				break
			}
			if w := runewidth.StringWidth(text); w > m.widths[c] {
				m.widths[c] = min(w, maxColWidth)
			}
		}
	}
}

func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch key.String() {
	case "up", "k":
		m.moveRow(-1)
	case "down", "j":
		m.moveRow(1)
	case "left", "h":
		m.moveCol(-1)
	case "right", "l":
		m.moveCol(1)
	case "home":
		m.cursorCol = 0
	case "end":
		if len(m.widths) > 0 {
			m.cursorCol = len(m.widths) - 1
		}
	case "g":
		m.cursorRow = 0
	case "G":
		if len(m.cells) > 0 {
			m.cursorRow = len(m.cells) - 1
		}
	case "pgup":
		m.moveRow(-max(1, m.height-1))
	case "pgdown":
		m.moveRow(max(1, m.height-1))
	case "[":
		m.adjustWidth(-2)
	case "]":
		m.adjustWidth(2)
	case "o":
		m.toggleSort()
	case "O":
		m.sortCol = -1
		m.sortAsc = true
		m.resetOrder()
	}
	m.ensureVisible()
	return *m, nil
}

func (m *Model) moveRow(delta int) {
	if len(m.cells) == 0 {
		return
	}
	m.cursorRow = min(max(m.cursorRow+delta, 0), len(m.cells)-1)
}

func (m *Model) moveCol(delta int) {
	if len(m.widths) == 0 {
		return
	}
	m.cursorCol = min(max(m.cursorCol+delta, 0), len(m.widths)-1)
}

func (m *Model) adjustWidth(delta int) {
	if m.cursorCol < 0 || m.cursorCol >= len(m.widths) {
		return
	}
	m.widths[m.cursorCol] = min(max(m.widths[m.cursorCol]+delta, minColWidth), 80)
	m.ensureVisible()
}

func (m *Model) toggleSort() {
	if m.rs == nil || m.cursorCol >= len(m.widths) || len(m.cells) == 0 {
		return
	}
	if m.sortCol == m.cursorCol {
		m.sortAsc = !m.sortAsc
	} else {
		m.sortCol = m.cursorCol
		m.sortAsc = true
	}
	m.applyOrder()
}

func (m *Model) resetOrder() {
	m.order = make([]int, len(m.cells))
	for i := range m.order {
		m.order[i] = i
	}
	m.cursorRow = 0
	m.scrollRow = 0
}

func (m *Model) applyOrder() {
	m.resetOrder()
	if m.rs == nil || m.sortCol < 0 || m.sortCol >= len(m.widths) {
		return
	}
	col := m.sortCol
	sort.SliceStable(m.order, func(i, j int) bool {
		a := valueAt(m.rs.Rows, m.order[i], col)
		b := valueAt(m.rs.Rows, m.order[j], col)
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		cmp := compareValues(a, b)
		if m.sortAsc {
			return cmp < 0
		}
		return cmp > 0
	})
}

func valueAt(rows [][]any, row, col int) any {
	if row >= len(rows) || col >= len(rows[row]) {
		return nil
	}
	return rows[row][col]
}

func compareValues(a, b any) int {
	switch av := a.(type) {
	case int64:
		if bv, ok := b.(int64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case float64:
		if bv, ok := b.(float64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case bool:
		if bv, ok := b.(bool); ok {
			switch {
			case !av && bv:
				return -1
			case av && !bv:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

func (m Model) CursorCell() string {
	if len(m.cells) == 0 || len(m.widths) == 0 || m.cursorRow >= len(m.cells) {
		return ""
	}
	row := m.cursorRow
	if row < len(m.order) {
		row = m.order[row]
	}
	if m.cursorCol >= len(m.cells[row]) {
		return ""
	}
	return m.cells[row][m.cursorCol]
}

func (m *Model) ensureVisible() {
	if len(m.widths) > 0 {
		if m.cursorCol < m.scrollCol {
			m.scrollCol = m.cursorCol
		}
		start, end := m.visibleRange()
		for m.cursorCol >= end && m.scrollCol < m.cursorCol {
			m.scrollCol++
			start, end = m.visibleRange()
			if start > m.cursorCol {
				m.scrollCol = m.cursorCol
			}
		}
	}
	visible := max(1, m.height-1)
	if m.cursorRow < m.scrollRow {
		m.scrollRow = m.cursorRow
	}
	if m.cursorRow >= m.scrollRow+visible {
		m.scrollRow = m.cursorRow - visible + 1
	}
	if m.scrollRow < 0 {
		m.scrollRow = 0
	}
}

func (m *Model) visibleRange() (int, int) {
	if len(m.widths) == 0 {
		return 0, 0
	}
	start := min(m.scrollCol, len(m.widths)-1)
	used := rowNumWidth
	end := start
	for end < len(m.widths) {
		avail := m.width - used
		if avail <= 1 {
			break
		}
		w := min(m.widths[end], avail-1)
		used += w + 1
		end++
	}
	if end == start {
		end = start + 1
	}
	return start, end
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.errText != "" {
		return textutil.FitLines(theme.Error.Render(m.errText), m.width, m.height)
	}
	if m.affected != nil {
		line := fmt.Sprintf("%d row(s) affected in %s", *m.affected, humanDuration(m.elapsed))
		return textutil.FitLines(theme.Muted.Render(line), m.width, m.height)
	}
	if m.rs == nil {
		return textutil.FitLines(theme.Muted.Render(m.t.T(i18n.KeyResultsPrompt)), m.width, m.height)
	}
	if m.height < 2 {
		return ""
	}

	m.ensureVisible()
	lines := make([]string, 0, m.height)
	lines = append(lines, m.renderHeader())
	if len(m.cells) == 0 {
		lines = append(lines, textutil.FitLine(theme.Muted.Render("  "+m.t.T(i18n.KeyNoRows)), m.width))
	}
	for r := m.scrollRow; r < len(m.cells) && len(lines) < m.height; r++ {
		lines = append(lines, m.renderRow(r, r == m.cursorRow))
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderHeader() string {
	var b strings.Builder
	b.WriteString(theme.Header.Render(textutil.FitLine("#", rowNumWidth)))
	used := rowNumWidth
	for c := m.scrollCol; c < len(m.widths) && used < m.width; c++ {
		avail := m.width - used
		if avail <= 1 {
			break
		}
		w := min(m.widths[c], avail-1)
		name := m.rs.Columns[c].Name
		if c == m.sortCol && m.sortCol >= 0 {
			if m.sortAsc {
				name += " ↑"
			} else {
				name += " ↓"
			}
		}
		b.WriteString(" ")
		b.WriteString(theme.Header.Render(textutil.FitLine(textutil.Truncate(name, w), w)))
		used += w + 1
	}
	if used < m.width {
		b.WriteString(strings.Repeat(" ", m.width-used))
	}
	return b.String()
}

func (m *Model) renderRow(r int, selected bool) string {
	var b strings.Builder
	rowNumStyle := lipgloss.NewStyle()
	if selected {
		rowNumStyle = theme.Selected
	}
	actual := r
	if r < len(m.order) {
		actual = m.order[r]
	}
	if actual >= len(m.cells) {
		return ""
	}
	b.WriteString(rowNumStyle.Render(fmt.Sprintf("%*d", rowNumWidth, actual+1)))
	used := rowNumWidth
	row := m.cells[actual]
	for c := m.scrollCol; c < len(m.widths) && used < m.width; c++ {
		avail := m.width - used
		if avail <= 1 {
			break
		}
		w := min(m.widths[c], avail-1)
		text := ""
		if c < len(row) {
			text = row[c]
		}
		style := lipgloss.NewStyle()
		switch {
		case selected && c == m.cursorCol:
			style = theme.CellCursor
		case selected:
			style = theme.Selected
		case text == nullText:
			style = theme.Null
		}
		b.WriteString(" ")
		b.WriteString(style.Render(textutil.FitLine(textutil.Truncate(text, w), w)))
		used += w + 1
	}
	if used < m.width {
		b.WriteString(strings.Repeat(" ", m.width-used))
	}
	return b.String()
}

func displayValue(v any) string {
	if v == nil {
		return nullText
	}
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case []byte:
		s = string(x)
	case time.Time:
		s = x.Format(time.RFC3339)
	default:
		s = fmt.Sprint(x)
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

func humanDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}
