package results

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
)

const (
	minColWidth = 4
	maxColWidth = 40
	cellPadding = 2
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
	rowMode              bool

	sourceSchema string
	sourceTable  string
	querySQL     string
	pageInfo     string

	order   []int
	visible []int
	filter  string
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
	m.sourceSchema, m.sourceTable = "", ""
	m.querySQL, m.pageInfo = "", ""
	m.errText = ""
	m.affected = nil
	m.elapsed = rs.Elapsed
	m.cursorRow, m.cursorCol = 0, 0
	m.scrollRow, m.scrollCol = 0, 0
	m.sortCol = -1
	m.sortAsc = true
	m.filter = ""
	m.visible = nil
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
	m.applyFilter()
	m.computeWidths()
}

func (m *Model) SetExec(affected int64, elapsed time.Duration) {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.querySQL, m.pageInfo = "", ""
	m.errText = ""
	m.affected = &affected
	m.elapsed = elapsed
	m.filter = ""
	m.order, m.visible = nil, nil
}

func (m *Model) SetError(text string) {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.querySQL, m.pageInfo = "", ""
	m.affected = nil
	m.errText = text
	m.filter = ""
	m.order, m.visible = nil, nil
}

func (m *Model) SetPage(sql, info string) {
	m.querySQL = sql
	m.pageInfo = info
}

func (m *Model) Result() *db.ResultSet { return m.rs }

// OrderedResult returns a copy of the result with rows in the currently
// displayed order, so exports respect client-side sorting.
func (m Model) OrderedResult() *db.ResultSet {
	if m.rs == nil {
		return nil
	}
	out := *m.rs
	rows := make([][]any, len(m.visible))
	for i, index := range m.visible {
		rows[i] = m.rs.Rows[index]
	}
	out.Rows = rows
	return &out
}

func (m *Model) Clear() {
	m.rs = nil
	m.cells = nil
	m.widths = nil
	m.querySQL, m.pageInfo = "", ""
	m.affected = nil
	m.errText = ""
	m.cursorRow, m.cursorCol = 0, 0
	m.scrollRow, m.scrollCol = 0, 0
	m.order = nil
	m.visible = nil
	m.filter = ""
	m.sortCol = -1
	m.sortAsc = true
}

// SetFilter keeps only rows that contain text in any column. Matching is
// case-insensitive and applies to the displayed strings.
func (m *Model) SetFilter(text string) {
	m.filter = text
	m.applyFilter()
	m.cursorRow = 0
	m.scrollRow = 0
	m.ensureVisible()
}

func (m Model) Filter() string { return m.filter }

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.visible = append(m.visible[:0], m.order...)
		return
	}
	needle := strings.ToLower(m.filter)
	m.visible = m.visible[:0]
	for _, index := range m.order {
		if index < 0 || index >= len(m.cells) {
			continue
		}
		for _, cell := range m.cells[index] {
			if strings.Contains(strings.ToLower(cell), needle) {
				m.visible = append(m.visible, index)
				break
			}
		}
	}
}

func (m *Model) computeWidths() {
	if m.rs == nil {
		return
	}
	m.widths = make([]int, len(m.rs.Columns))
	for i, col := range m.rs.Columns {
		m.widths[i] = min(max(runewidth.StringWidth(col.Name)+cellPadding, minColWidth), maxColWidth)
	}
	sample := min(len(m.cells), 200)
	for r := 0; r < sample; r++ {
		for c, text := range m.cells[r] {
			if c >= len(m.widths) {
				break
			}
			if w := runewidth.StringWidth(text) + cellPadding; w > m.widths[c] {
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
		if !m.rowMode {
			m.cursorCol = 0
		}
	case "end":
		if !m.rowMode && len(m.widths) > 0 {
			m.cursorCol = len(m.widths) - 1
		}
	case "g":
		m.cursorRow = 0
	case "G":
		if len(m.visible) > 0 {
			m.cursorRow = len(m.visible) - 1
		}
	case "pgup":
		m.moveRow(-max(1, m.bodyRows()))
	case "pgdown":
		m.moveRow(max(1, m.bodyRows()))
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
	case "H":
		m.ScrollHorizontal(-1)
	case "L":
		m.ScrollHorizontal(1)
	}
	m.ensureVisible()
	return *m, nil
}

func (m *Model) moveRow(delta int) {
	if len(m.visible) == 0 {
		return
	}
	m.cursorRow = min(max(m.cursorRow+delta, 0), len(m.visible)-1)
}

func (m *Model) moveCol(delta int) {
	if m.rowMode || len(m.widths) == 0 {
		return
	}
	m.cursorCol = min(max(m.cursorCol+delta, 0), len(m.widths)-1)
}

func (m *Model) adjustWidth(delta int) {
	if m.cursorCol < 0 || m.cursorCol >= len(m.widths) {
		return
	}
	m.widths[m.cursorCol] = min(max(m.widths[m.cursorCol]+delta, minColWidth), 120)
	m.ensureVisible()
}

func (m *Model) ScrollHorizontal(delta int) {
	if len(m.widths) == 0 {
		return
	}
	step := max(1, len(m.columnLayout()))
	m.scrollCol = min(max(m.scrollCol+delta*step, 0), len(m.widths)-1)
	m.cursorCol = m.scrollCol
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
	m.applyFilter()
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
	m.applyFilter()
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
	if len(m.cells) == 0 || len(m.widths) == 0 || m.cursorRow >= len(m.visible) {
		return ""
	}
	row := m.visible[m.cursorRow]
	if m.cursorCol >= len(m.cells[row]) {
		return ""
	}
	return m.cells[row][m.cursorCol]
}

// CursorValue returns the raw value under the cursor together with whether a
// cell exists. Unlike CursorCell it preserves NULL, newlines and other control
// characters so callers can edit the value safely.
func (m Model) CursorValue() (any, bool) {
	if m.rs == nil || len(m.cells) == 0 || m.cursorRow >= len(m.visible) {
		return nil, false
	}
	row := m.visible[m.cursorRow]
	if row >= len(m.rs.Rows) || m.cursorCol >= len(m.rs.Rows[row]) {
		return nil, false
	}
	return m.rs.Rows[row][m.cursorCol], true
}

func (m Model) CursorPosition() (row, col int) { return m.cursorRow, m.cursorCol }

func (m *Model) SetSource(schema, table string) {
	m.sourceSchema, m.sourceTable = schema, table
}

func (m *Model) SetSort(col int, asc bool) {
	m.sortCol = col
	m.sortAsc = asc
}

func (m Model) ColumnIndex(name string) int {
	if m.rs == nil {
		return -1
	}
	for i, col := range m.rs.Columns {
		if col.Name == name {
			return i
		}
	}
	return -1
}

func (m Model) Source() (schema, table string, ok bool) {
	if m.sourceTable == "" {
		return "", "", false
	}
	return m.sourceSchema, m.sourceTable, true
}

func (m Model) RowKey(columns []string) (map[string]any, bool) {
	if m.rs == nil || len(columns) == 0 || m.cursorRow < 0 || m.cursorRow >= len(m.visible) {
		return nil, false
	}
	row := m.visible[m.cursorRow]
	if row >= len(m.rs.Rows) {
		return nil, false
	}
	key := make(map[string]any, len(columns))
	for _, name := range columns {
		index := -1
		for i, column := range m.rs.Columns {
			if column.Name == name {
				index = i
				break
			}
		}
		if index < 0 || index >= len(m.rs.Rows[row]) {
			return nil, false
		}
		key[name] = m.rs.Rows[row][index]
	}
	return key, true
}

func (m Model) CursorColumnName() string {
	if m.rs == nil || m.cursorCol < 0 || m.cursorCol >= len(m.rs.Columns) {
		return ""
	}
	return m.rs.Columns[m.cursorCol].Name
}

// CursorRow returns the raw values of the selected row in display order.
func (m Model) CursorRow() ([]any, bool) {
	if m.rs == nil || m.cursorRow < 0 || m.cursorRow >= len(m.visible) {
		return nil, false
	}
	row := m.visible[m.cursorRow]
	if row < 0 || row >= len(m.rs.Rows) {
		return nil, false
	}
	return m.rs.Rows[row], true
}

// SetRowMode disables column selection and highlights whole rows. Used by
// read-only panels such as the table overview.
func (m *Model) SetRowMode(enabled bool) {
	m.rowMode = enabled
	if enabled {
		m.cursorCol = 0
		m.scrollCol = 0
	}
}

func (m Model) CursorText() string {
	if m.rs == nil || len(m.widths) == 0 || m.cursorRow >= len(m.visible) {
		return ""
	}
	row := m.visible[m.cursorRow]
	if row >= len(m.rs.Rows) || m.cursorCol >= len(m.rs.Rows[row]) {
		return ""
	}
	return rawText(m.rs.Rows[row][m.cursorCol])
}

func rawText(v any) string {
	switch x := v.(type) {
	case nil:
		return nullText
	case string:
		return x
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(x)
	}
}

func (m *Model) HandleClick(x, y int) bool {
	if y < 3 || len(m.visible) == 0 {
		return false
	}
	row := m.scrollRow + (y - 3)
	if row < 0 || row >= len(m.visible) {
		return false
	}
	cursorX := 1
	for _, col := range m.columnLayout() {
		if x >= cursorX && x < cursorX+col.width {
			m.cursorCol = col.index
			m.cursorRow = row
			m.ensureVisible()
			return true
		}
		cursorX += col.width + 1
	}
	return false
}

func (m *Model) Scroll(delta int) {
	m.moveRow(delta * 3)
	m.ensureVisible()
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
	visible := max(1, m.bodyRows())
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

func (m *Model) bodyRows() int {
	return max(0, m.height-4-m.footerHeight())
}

func (m *Model) footerHeight() int {
	if m.querySQL != "" || m.pageInfo != "" || m.filter != "" {
		return 1
	}
	return 0
}

func (m *Model) footerLines() []string {
	line := m.footerLine()
	if line == "" {
		return nil
	}
	return []string{line}
}

func (m *Model) footerLine() string {
	if m.querySQL == "" && m.pageInfo == "" && m.filter == "" {
		return ""
	}
	sqlPart := ""
	if m.querySQL != "" {
		sqlPart = m.t.T(i18n.KeyResultsSQL) + " " + m.querySQL
	}
	meta := m.pageInfo
	if m.filter != "" {
		filtered := fmt.Sprintf(m.t.T(i18n.KeyResultsFiltered), len(m.visible), len(m.cells))
		if meta == "" {
			meta = filtered
		} else {
			meta = filtered + " · " + meta
		}
	}
	if sqlPart == "" {
		return theme.Footer.Render(textutil.FitLine(" "+footerTruncate(meta, max(0, m.width-1)), m.width))
	}
	if meta == "" {
		return theme.Footer.Render(textutil.FitLine(" "+footerTruncate(sqlPart, max(0, m.width-1)), m.width))
	}
	if metaWidth := textutil.Width(meta); metaWidth > (m.width-2)/2 {
		meta = footerTruncate(meta, max(0, (m.width-2)/2))
	}
	available := m.width - textutil.Width(meta) - 2
	sqlPart = footerTruncate(sqlPart, max(0, available))
	gap := max(1, m.width-1-textutil.Width(sqlPart)-textutil.Width(meta))
	line := " " + sqlPart + strings.Repeat(" ", gap) + meta
	return theme.Footer.Render(textutil.FitLine(line, m.width))
}

func footerTruncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if textutil.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return textutil.Truncate(text, width-1) + "…"
}

type column struct {
	index int
	width int
}

func (m *Model) columnLayout() []column {
	if len(m.widths) == 0 {
		return nil
	}
	start := min(max(m.scrollCol, 0), len(m.widths)-1)
	used := 1
	out := make([]column, 0, len(m.widths)-start)
	for c := start; c < len(m.widths); c++ {
		avail := m.width - used - 1
		if avail < 1 {
			break
		}
		w := min(m.widths[c], avail)
		out = append(out, column{index: c, width: w})
		used += w + 1
	}
	if len(out) == 0 {
		w := max(1, m.width-used-1)
		out = append(out, column{index: start, width: w})
		used = 1 + w + 1
	}
	if remainder := m.width - used; remainder > 0 {
		out[len(out)-1].width += remainder
	}
	return out
}

func (m *Model) visibleRange() (int, int) {
	layout := m.columnLayout()
	if len(layout) == 0 {
		return 0, 0
	}
	return layout[0].index, layout[len(layout)-1].index + 1
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.errText != "" {
		return textutil.FitLines(theme.Error.Render(m.errText), m.width, m.height)
	}
	if m.affected != nil {
		line := fmt.Sprintf(m.t.T(i18n.KeyResultsAffected), *m.affected, humanDuration(m.elapsed))
		return textutil.FitLines(theme.Muted.Render(line), m.width, m.height)
	}
	if m.rs == nil {
		return textutil.FitLines(theme.Muted.Render(m.t.T(i18n.KeyResultsPrompt)), m.width, m.height)
	}
	footer := m.footerLines()
	gridHeight := m.height - len(footer)
	if gridHeight < 4 {
		content := theme.Muted.Render(m.t.T(i18n.KeyNoRows))
		if len(footer) > 0 {
			content += "\n" + strings.Join(footer, "\n")
		}
		return textutil.FitLines(content, m.width, m.height)
	}

	m.ensureVisible()
	layout := m.columnLayout()
	lines := make([]string, 0, m.height)
	lines = append(lines, m.borderLine(layout, borderTop))
	lines = append(lines, m.renderHeader(layout))
	lines = append(lines, m.borderLine(layout, borderMid))

	if len(m.visible) == 0 {
		if m.bodyRows() > 0 {
			lines = append(lines, m.messageRow(m.t.T(i18n.KeyNoRows)))
		}
	} else {
		for r := m.scrollRow; r < len(m.visible) && len(lines) < gridHeight-1; r++ {
			lines = append(lines, m.renderRow(layout, r, r == m.cursorRow))
		}
	}
	for len(lines) < gridHeight-1 {
		lines = append(lines, m.emptyRow(layout))
	}
	lines = append(lines, m.borderLine(layout, borderBottom))
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

type borderKind int

const (
	borderTop borderKind = iota
	borderMid
	borderBottom
)

func (m *Model) borderLine(layout []column, kind borderKind) string {
	left, join, right := "┌", "┬", "┐"
	switch kind {
	case borderMid:
		left, join, right = "├", "┼", "┤"
	case borderBottom:
		left, join, right = "└", "┴", "┘"
	}

	var b strings.Builder
	b.WriteString(left)
	for i, col := range layout {
		b.WriteString(strings.Repeat("─", col.width))
		if i == len(layout)-1 {
			b.WriteString(right)
		} else {
			b.WriteString(join)
		}
	}
	if len(layout) == 0 {
		b.WriteString(right)
	}
	return theme.Muted.Render(textutil.FitLine(b.String(), m.width))
}

func (m *Model) messageRow(text string) string {
	inner := max(0, m.width-2)
	return "│" + theme.Muted.Render(textutil.FitLine(" "+text, inner)) + "│"
}

func (m *Model) emptyRow(layout []column) string {
	var b strings.Builder
	b.WriteString("│")
	for _, col := range layout {
		b.WriteString(strings.Repeat(" ", col.width))
		b.WriteString("│")
	}
	return b.String()
}

func (m *Model) renderHeader(layout []column) string {
	var b strings.Builder
	b.WriteString("│")
	for _, col := range layout {
		name := m.rs.Columns[col.index].Name
		if col.index == m.sortCol && m.sortCol >= 0 {
			if m.sortAsc {
				name += "↑"
			} else {
				name += "↓"
			}
		}
		b.WriteString(theme.Header.Render(textutil.FitLine(" "+name, col.width)))
		b.WriteString("│")
	}
	return b.String()
}

func (m *Model) renderRow(layout []column, r int, selected bool) string {
	if r < 0 || r >= len(m.visible) {
		return m.emptyRow(layout)
	}
	actual := m.visible[r]
	if actual >= len(m.cells) {
		return m.emptyRow(layout)
	}
	row := m.cells[actual]

	var b strings.Builder
	b.WriteString("│")
	for _, col := range layout {
		text := ""
		if col.index < len(row) {
			text = row[col.index]
		}
		style := lipgloss.NewStyle()
		switch {
		case selected && !m.rowMode && col.index == m.cursorCol:
			style = theme.CellCursor
		case selected:
			style = theme.Selected
		case text == nullText:
			style = theme.Null
		}
		b.WriteString(style.Render(textutil.FitLine(" "+text, col.width)))
		b.WriteString("│")
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
