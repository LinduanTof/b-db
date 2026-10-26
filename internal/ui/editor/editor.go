package editor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/LinduanTof/dbrix/internal/complete"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/ui/textutil"
	"github.com/LinduanTof/dbrix/internal/ui/theme"
)

const maxHistory = 200

type Model struct {
	t *i18n.Translator

	ta        textarea.Model
	width     int
	height    int
	scrollRow int

	history []string
	histPos int

	lastCursorRow int

	cacheValue  string
	cacheWidth  int
	cacheLines  []string
	cacheTokens [][]token
	cacheBlock  []bool
	cacheRows   []int
	cacheStart  []int

	findOpen    bool
	findQuery   string
	findMatches []findMatch
	findCurrent int
}

type findMatch struct {
	line   int
	start  int
	length int
}

func New(t *i18n.Translator) Model {
	ta := textarea.New()
	ta.Prompt = "› "
	ta.Placeholder = "SELECT * FROM ..."
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	return Model{t: t, ta: ta, lastCursorRow: -1}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.ta.SetWidth(max(width, 1))
	m.ta.SetHeight(max(height, 1))
	m.lastCursorRow = -1
}

func (m *Model) Focus() tea.Cmd { return m.ta.Focus() }

func (m *Model) Blur() { m.ta.Blur() }

func (m Model) Value() string { return m.ta.Value() }

// CursorOffset returns the cursor position as a rune offset into Value.
func (m Model) CursorOffset() int {
	line := m.ta.Line()
	col := max(0, m.ta.Column())
	if line <= 0 {
		return col
	}
	lines := strings.Split(m.ta.Value(), "\n")
	offset := 0
	for i := 0; i < line && i < len(lines); i++ {
		offset += len([]rune(lines[i])) + 1
	}
	return offset + col
}

func (m *Model) SetValue(s string) {
	m.ta.SetValue(s)
	m.ta.CursorEnd()
	m.scrollRow = 0
	m.lastCursorRow = -1
}

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

// visualRow is one screen line inside a logical line. Long lines are wrapped
// at the editor width so values never run past the pane border.
type visualRow struct {
	line       int
	start, end int
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	value := m.ta.Value()
	lineCount := strings.Count(value, "\n") + 1
	gutter := len(strconv.Itoa(lineCount)) + 1
	textWidth := max(1, m.width-gutter)
	lines, tokens, starts := m.prepare(value, textWidth)

	cursorLine := m.ta.Line()
	cursorCol := m.ta.Column()
	if cursorLine < 0 {
		cursorLine = 0
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}
	cursorSegment := segmentForCol([]rune(lines[cursorLine]), cursorCol, textWidth)
	cursorRow := starts[cursorLine] + cursorSegment
	total := starts[len(lines)]

	// Follow the cursor only when it moves. Mouse-wheel scrolling changes
	// scrollRow without touching the cursor, so the view must stay put.
	if cursorRow != m.lastCursorRow {
		if cursorRow < m.scrollRow {
			m.scrollRow = cursorRow
		}
		if cursorRow >= m.scrollRow+m.height {
			m.scrollRow = cursorRow - m.height + 1
		}
		m.lastCursorRow = cursorRow
	}
	m.scrollRow = min(max(m.scrollRow, 0), max(0, total-m.height))

	showCursor := m.ta.Focused()
	out := make([]string, 0, m.height)
	if value == "" {
		out = append(out, m.placeholderLine(gutter, showCursor))
	} else {
		line := lineIndexAt(starts, m.scrollRow)
		segment := m.scrollRow - starts[line]
		for line < len(lines) && len(out) < m.height {
			bounds := segmentBoundsAll([]rune(lines[line]), textWidth)
			for segment < len(bounds) && len(out) < m.height {
				row := visualRow{line: line, start: bounds[segment][0], end: bounds[segment][1]}
				prefix := strings.Repeat(" ", gutter)
				if row.start == 0 {
					prefix = theme.Muted.Render(fmt.Sprintf("%*d ", gutter-1, line+1))
				}
				out = append(out, m.renderSegment(tokens[line], row, prefix,
					cursorLine, cursorCol, showCursor && line == cursorLine && segment == cursorSegment))
				segment++
			}
			line++
			segment = 0
		}
	}
	for len(out) < m.height {
		out = append(out, "")
	}
	return textutil.FitLines(strings.Join(out, "\n"), m.width, m.height)
}

// visualRowCount returns how many screen rows a logical line occupies.
func visualRowCount(runes []rune, width int) int {
	if len(runes) == 0 {
		return 1
	}
	count, start, used := 1, 0, 0
	for i, r := range runes {
		w := runewidth.RuneWidth(r)
		if used+w > width && i > start {
			count++
			start = i
			used = 0
		}
		used += w
	}
	return count
}

// segmentBoundsAll splits a logical line into screen rows.
func segmentBoundsAll(runes []rune, width int) [][2]int {
	if len(runes) == 0 {
		return [][2]int{{0, 0}}
	}
	out := make([][2]int, 0, 1)
	start, used := 0, 0
	for i, r := range runes {
		w := runewidth.RuneWidth(r)
		if used+w > width && i > start {
			out = append(out, [2]int{start, i})
			start = i
			used = 0
		}
		used += w
	}
	return append(out, [2]int{start, len(runes)})
}

// segmentForCol returns the screen row that holds a rune column.
func segmentForCol(runes []rune, col, width int) int {
	if len(runes) == 0 {
		return 0
	}
	segment, start, used := 0, 0, 0
	for i, r := range runes {
		w := runewidth.RuneWidth(r)
		if used+w > width && i > start {
			if col < i {
				return segment
			}
			segment++
			start = i
			used = 0
		}
		used += w
	}
	return segment
}

// lineIndexAt finds the logical line that contains a visual row.
func lineIndexAt(starts []int, row int) int {
	if row <= 0 || len(starts) < 2 {
		return 0
	}
	index := sort.Search(len(starts)-1, func(i int) bool { return starts[i+1] > row })
	if index >= len(starts)-1 {
		index = len(starts) - 2
	}
	return max(0, index)
}

func visualRows(lines []string, width int) []visualRow {
	rows := make([]visualRow, 0, len(lines))
	for i, line := range lines {
		for _, bound := range segmentBoundsAll([]rune(line), width) {
			rows = append(rows, visualRow{line: i, start: bound[0], end: bound[1]})
		}
	}
	return rows
}

// prepare returns the cached line layout. Only lines that changed are
// re-tokenized and re-measured, so typing does not rescan the whole document.
func (m *Model) prepare(value string, width int) ([]string, [][]token, []int) {
	lines := strings.Split(value, "\n")
	if value == "" {
		lines = []string{""}
	}
	if m.cacheLines != nil && m.cacheValue == value && m.cacheWidth == width {
		return m.cacheLines, m.cacheTokens, m.cacheStart
	}

	oldLines := m.cacheLines
	oldTokens := m.cacheTokens
	oldBlock := m.cacheBlock
	oldRows := m.cacheRows
	widthChanged := oldLines == nil || m.cacheWidth != width

	prefix, suffix := 0, 0
	if !widthChanged {
		limit := min(len(oldLines), len(lines))
		for prefix < limit && oldLines[prefix] == lines[prefix] {
			prefix++
		}
		if prefix < len(lines) {
			maxSuffix := min(len(lines)-prefix, len(oldLines)-prefix)
			for suffix < maxSuffix &&
				lines[len(lines)-1-suffix] == oldLines[len(oldLines)-1-suffix] {
				suffix++
			}
		}
	}

	tokens := make([][]token, len(lines))
	block := make([]bool, len(lines))
	rowCounts := make([]int, len(lines))

	for i := 0; i < prefix; i++ {
		tokens[i] = oldTokens[i]
		block[i] = oldBlock[i]
	}
	inBlock := prefix > 0 && block[prefix-1]
	changedEnd := len(lines) - suffix
	for i := prefix; i < changedEnd; i++ {
		tokens[i] = tokenizeLine(lines[i], &inBlock)
		block[i] = inBlock
	}
	if suffix > 0 {
		oldStart := len(oldLines) - suffix
		oldState := oldStart > 0 && oldBlock[oldStart-1]
		if inBlock == oldState {
			for i := 0; i < suffix; i++ {
				tokens[changedEnd+i] = oldTokens[oldStart+i]
				block[changedEnd+i] = oldBlock[oldStart+i]
			}
		} else {
			for i := changedEnd; i < len(lines); i++ {
				tokens[i] = tokenizeLine(lines[i], &inBlock)
				block[i] = inBlock
			}
		}
	}

	if widthChanged {
		for i, line := range lines {
			rowCounts[i] = visualRowCount([]rune(line), width)
		}
	} else {
		for i := 0; i < changedEnd; i++ {
			if i < prefix {
				rowCounts[i] = oldRows[i]
			} else {
				rowCounts[i] = visualRowCount([]rune(lines[i]), width)
			}
		}
		oldStart := len(oldLines) - suffix
		for i := 0; i < suffix; i++ {
			rowCounts[changedEnd+i] = oldRows[oldStart+i]
		}
	}

	starts := make([]int, len(lines)+1)
	for i, count := range rowCounts {
		starts[i+1] = starts[i] + count
	}
	m.cacheValue = value
	m.cacheWidth = width
	m.cacheLines = lines
	m.cacheTokens = tokens
	m.cacheBlock = block
	m.cacheRows = rowCounts
	m.cacheStart = starts
	return lines, tokens, starts
}

// visualIndexFor maps a cursor position to the screen line that holds it.
// A column at a wrap boundary belongs to the following row; a column at the
// end of the logical line belongs to the last row of that line.
func visualIndexFor(rows []visualRow, line, col int) int {
	last := 0
	for i, row := range rows {
		if row.line != line {
			continue
		}
		last = i
		if col >= row.start && col < row.end {
			return i
		}
	}
	return last
}

// HandleClick moves the cursor to a screen position inside the editor body.
// x is relative to the pane's left edge and y to the top of View.
func (m *Model) HandleClick(x, y int) bool {
	if m.width <= 0 || m.height <= 0 || y < 0 {
		return false
	}
	value := m.ta.Value()
	lineCount := strings.Count(value, "\n") + 1
	gutter := len(strconv.Itoa(lineCount)) + 1
	textWidth := max(1, m.width-gutter)
	lines, _, starts := m.prepare(value, textWidth)
	total := starts[len(lines)]
	index := m.scrollRow + y
	if index < 0 || index >= total {
		return false
	}
	line := lineIndexAt(starts, index)
	segment := index - starts[line]
	runes := []rune(lines[line])
	bounds := segmentBoundsAll(runes, textWidth)
	segment = min(max(segment, 0), len(bounds)-1)
	start, end := bounds[segment][0], bounds[segment][1]
	col := start
	if x > gutter {
		textCol := x - gutter
		used := 0
		for col < end {
			w := runewidth.RuneWidth(runes[col])
			if used+w > textCol {
				break
			}
			used += w
			col++
		}
	}
	m.setCursor(line, col)
	m.lastCursorRow = -1
	return true
}

// CursorScreen returns the cursor position relative to the editor body (the
// same coordinates as View) and whether it is currently visible. The column
// includes the line-number gutter.
func (m *Model) CursorScreen() (int, int, bool) {
	if m.width <= 0 || m.height <= 0 {
		return 0, 0, false
	}
	value := m.ta.Value()
	lineCount := strings.Count(value, "\n") + 1
	gutter := len(strconv.Itoa(lineCount)) + 1
	textWidth := max(1, m.width-gutter)
	lines, _, starts := m.prepare(value, textWidth)

	cursorLine := min(max(m.ta.Line(), 0), len(lines)-1)
	runes := []rune(lines[cursorLine])
	cursorCol := min(max(m.ta.Column(), 0), len(runes))
	segment := segmentForCol(runes, cursorCol, textWidth)
	row := starts[cursorLine] + segment - m.scrollRow
	if row < 0 || row >= m.height {
		return 0, 0, false
	}

	bounds := segmentBoundsAll(runes, textWidth)
	start := bounds[min(segment, len(bounds)-1)][0]
	col := gutter
	for i := start; i < cursorCol; i++ {
		col += runewidth.RuneWidth(runes[i])
	}
	return row, col, true
}

// Scroll moves the viewport without moving the cursor.
func (m *Model) Scroll(delta int) {
	if m.width <= 0 || m.height <= 0 || delta == 0 {
		return
	}
	value := m.ta.Value()
	lineCount := strings.Count(value, "\n") + 1
	gutter := len(strconv.Itoa(lineCount)) + 1
	textWidth := max(1, m.width-gutter)
	lines, _, starts := m.prepare(value, textWidth)
	total := starts[len(lines)]
	cursorLine := min(max(m.ta.Line(), 0), len(lines)-1)
	// Record where the cursor is so the next View keeps this scroll offset
	// instead of snapping back before the first render.
	m.lastCursorRow = starts[cursorLine] +
		segmentForCol([]rune(lines[cursorLine]), m.ta.Column(), textWidth)
	maxScroll := max(0, total-m.height)
	m.scrollRow = min(max(m.scrollRow+delta*3, 0), maxScroll)
}

func (m *Model) setCursor(line, col int) {
	// CursorUp/CursorDown move by visual row once the textarea wraps, so a
	// wrapped long line looks like the end of the document. Widen the
	// textarea while moving so each logical line is exactly one row.
	width := max(1, m.width)
	m.ta.SetWidth(1 << 16)
	current := m.ta.Line()
	for current < line {
		m.ta.CursorDown()
		next := m.ta.Line()
		if next <= current {
			break
		}
		current = next
	}
	for current > line {
		m.ta.CursorUp()
		next := m.ta.Line()
		if next >= current {
			break
		}
		current = next
	}
	m.ta.SetCursorColumn(col)
	m.ta.SetWidth(width)
}

func (m *Model) placeholderLine(gutter int, showCursor bool) string {
	line := theme.Muted.Render(fmt.Sprintf("%*d ", gutter-1, 1)) +
		theme.Muted.Render(m.ta.Placeholder)
	if showCursor {
		line += theme.CellCursor.Render(" ")
	}
	return textutil.FitLine(line, m.width)
}

func (m *Model) renderSegment(tokens []token, row visualRow, prefix string, cursorLine, cursorCol int, showCursor bool) string {
	var b strings.Builder
	b.WriteString(prefix)

	var seg strings.Builder
	segKind := tokPlain
	segOverride := -1
	flush := func() {
		if seg.Len() == 0 {
			return
		}
		style := styleFor(segKind)
		switch segOverride {
		case 1:
			style = theme.Selected
		case 2, 3:
			style = theme.CellCursor
		}
		b.WriteString(style.Render(seg.String()))
		seg.Reset()
	}

	offset := 0
	cursorPlaced := false
	for _, tok := range tokens {
		if offset >= row.end {
			break
		}
		for _, r := range tok.text {
			if offset >= row.end {
				break
			}
			if offset < row.start {
				offset++
				continue
			}
			override := m.matchOverride(row.line, offset)
			if showCursor && row.line == cursorLine && offset == cursorCol {
				override = 3
				cursorPlaced = true
			}
			if override != segOverride || tok.kind != segKind {
				flush()
				segKind = tok.kind
				segOverride = override
			}
			seg.WriteRune(r)
			offset++
		}
	}
	flush()
	if showCursor && !cursorPlaced {
		b.WriteString(theme.CellCursor.Render(" "))
	}
	return textutil.FitLine(b.String(), m.width)
}

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

// OpenFind starts an incremental search over the current value. Matches are
// highlighted while the query is typed.
func (m *Model) OpenFind() {
	m.findOpen = true
	m.findQuery = ""
	m.findMatches = nil
	m.findCurrent = 0
}

// PasteFind appends pasted text to the find query.
func (m *Model) PasteFind(text string) {
	if !m.findOpen || text == "" {
		return
	}
	m.findQuery += text
	m.refreshFind()
}

func (m *Model) CloseFind() {
	m.findOpen = false
	m.findQuery = ""
	m.findMatches = nil
	m.findCurrent = 0
}

func (m Model) Finding() bool { return m.findOpen }

// FindStatus reports the query and the one-based position of the current
// match, for the editor pane hint.
func (m Model) FindStatus() (string, int, int) {
	if len(m.findMatches) == 0 {
		return m.findQuery, 0, 0
	}
	return m.findQuery, m.findCurrent + 1, len(m.findMatches)
}

// UpdateFind handles keys while the find prompt is open. It deliberately does
// not forward anything to the textarea.
func (m Model) UpdateFind(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !m.findOpen {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.CloseFind()
	case "enter", "down":
		m.nextMatch(1)
	case "shift+enter", "up":
		m.nextMatch(-1)
	case "backspace":
		runes := []rune(m.findQuery)
		if len(runes) > 0 {
			m.findQuery = string(runes[:len(runes)-1])
		}
		m.refreshFind()
	case "ctrl+u":
		m.findQuery = ""
		m.refreshFind()
	default:
		if msg.Text != "" {
			m.findQuery += msg.Text
			m.refreshFind()
		}
	}
	return m, nil
}

func (m *Model) refreshFind() {
	m.findMatches = nil
	m.findCurrent = 0
	if m.findQuery == "" {
		return
	}
	needle := []rune(m.findQuery)
	for lineIndex, line := range strings.Split(m.ta.Value(), "\n") {
		runes := []rune(line)
		for start := 0; start+len(needle) <= len(runes); start++ {
			if equalFoldRunes(runes[start:start+len(needle)], needle) {
				m.findMatches = append(m.findMatches, findMatch{line: lineIndex, start: start, length: len(needle)})
			}
		}
	}
	if len(m.findMatches) > 0 {
		m.jumpToMatch(0)
	}
}

func equalFoldRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(string(a[i]), string(b[i])) {
			return false
		}
	}
	return true
}

func (m *Model) nextMatch(delta int) {
	if len(m.findMatches) == 0 {
		return
	}
	m.jumpToMatch(m.findCurrent + delta)
}

func (m *Model) jumpToMatch(index int) {
	if len(m.findMatches) == 0 {
		return
	}
	count := len(m.findMatches)
	m.findCurrent = ((index % count) + count) % count
	match := m.findMatches[m.findCurrent]
	m.setCursor(match.line, match.start)
}

// matchOverride reports how a character should be styled: 1 for a match and 2
// for the current match.
func (m *Model) matchOverride(line, col int) int {
	if !m.findOpen || len(m.findMatches) == 0 {
		return 0
	}
	// refreshFind appends matches line by line, so only this line's matches
	// need to be checked.
	first := sort.Search(len(m.findMatches), func(i int) bool {
		return m.findMatches[i].line >= line
	})
	for i := first; i < len(m.findMatches) && m.findMatches[i].line == line; i++ {
		match := m.findMatches[i]
		if col >= match.start && col < match.start+match.length {
			if i == m.findCurrent {
				return 2
			}
			return 1
		}
	}
	return 0
}

type tokenKind int

const (
	tokPlain tokenKind = iota
	tokKeyword
	tokFunction
	tokString
	tokNumber
	tokComment
)

type token struct {
	text string
	kind tokenKind
}

var (
	sqlKeywordSet  = toSet(complete.Keywords())
	sqlFunctionSet = toSet(complete.Functions())
)

func toSet(words []string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, word := range words {
		out[strings.ToUpper(word)] = true
	}
	return out
}

func tokenizeLine(line string, inBlock *bool) []token {
	runes := []rune(line)
	out := make([]token, 0, 8)

	for i := 0; i < len(runes); {
		if *inBlock {
			start := i
			for i < len(runes) {
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					i += 2
					*inBlock = false
					break
				}
				i++
			}
			out = append(out, token{string(runes[start:i]), tokComment})
			continue
		}

		r := runes[i]
		switch {
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			out = append(out, token{string(runes[i:]), tokComment})
			i = len(runes)
		case r == '#':
			out = append(out, token{string(runes[i:]), tokComment})
			i = len(runes)
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			start := i
			i += 2
			*inBlock = true
			for i < len(runes) {
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					i += 2
					*inBlock = false
					break
				}
				i++
			}
			out = append(out, token{string(runes[start:i]), tokComment})
		case r == '\'' || r == '"' || r == '`':
			quote := r
			start := i
			i++
			for i < len(runes) {
				if runes[i] == quote {
					if i+1 < len(runes) && runes[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			out = append(out, token{string(runes[start:i]), tokString})
		case unicode.IsDigit(r):
			start := i
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			out = append(out, token{string(runes[start:i]), tokNumber})
		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			upper := strings.ToUpper(word)
			kind := tokPlain
			if sqlKeywordSet[upper] {
				kind = tokKeyword
			} else if sqlFunctionSet[upper] {
				kind = tokFunction
			}
			out = append(out, token{word, kind})
		default:
			out = append(out, token{string(r), tokPlain})
			i++
		}
	}
	return out
}

func styleFor(kind tokenKind) lipgloss.Style {
	switch kind {
	case tokKeyword:
		return theme.SyntaxKeyword
	case tokFunction:
		return theme.SyntaxFunction
	case tokString:
		return theme.SyntaxString
	case tokNumber:
		return theme.SyntaxNumber
	case tokComment:
		return theme.SyntaxComment
	default:
		return lipgloss.NewStyle()
	}
}
