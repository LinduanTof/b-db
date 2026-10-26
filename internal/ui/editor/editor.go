package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/LinduanTof/b-db/internal/complete"
	"github.com/LinduanTof/b-db/internal/i18n"
	"github.com/LinduanTof/b-db/internal/ui/textutil"
	"github.com/LinduanTof/b-db/internal/ui/theme"
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
	lines := strings.Split(value, "\n")
	if value == "" {
		lines = []string{""}
	}
	cursorLine := m.ta.Line()
	cursorCol := m.ta.Column()
	if cursorLine < 0 {
		cursorLine = 0
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}

	gutter := len(strconv.Itoa(max(1, len(lines)))) + 1
	rows := visualRows(lines, max(1, m.width-gutter))
	cursorRow := visualIndexFor(rows, cursorLine, cursorCol)

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
	m.scrollRow = min(max(m.scrollRow, 0), max(0, len(rows)-m.height))

	showCursor := m.ta.Focused()
	out := make([]string, 0, m.height)
	if value == "" {
		out = append(out, m.placeholderLine(gutter, showCursor))
	} else {
		inBlock := false
		tokens := make([][]token, len(lines))
		for i, line := range lines {
			tokens[i] = tokenizeLine(line, &inBlock)
		}
		for i := m.scrollRow; i < len(rows) && len(out) < m.height; i++ {
			row := rows[i]
			prefix := strings.Repeat(" ", gutter)
			if row.start == 0 {
				prefix = theme.Muted.Render(fmt.Sprintf("%*d ", gutter-1, row.line+1))
			}
			out = append(out, m.renderSegment(tokens[row.line], row, prefix,
				cursorLine, cursorCol, showCursor && i == cursorRow))
		}
	}
	for len(out) < m.height {
		out = append(out, "")
	}
	return textutil.FitLines(strings.Join(out, "\n"), m.width, m.height)
}

func visualRows(lines []string, width int) []visualRow {
	rows := make([]visualRow, 0, len(lines))
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			rows = append(rows, visualRow{line: i})
			continue
		}
		start, used := 0, 0
		for j, r := range runes {
			w := runewidth.RuneWidth(r)
			if used+w > width && j > start {
				rows = append(rows, visualRow{line: i, start: start, end: j})
				start = j
				used = 0
			}
			used += w
		}
		rows = append(rows, visualRow{line: i, start: start, end: len(runes)})
	}
	return rows
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
	lines := strings.Split(value, "\n")
	if value == "" {
		lines = []string{""}
	}
	gutter := len(strconv.Itoa(max(1, len(lines)))) + 1
	rows := visualRows(lines, max(1, m.width-gutter))
	index := m.scrollRow + y
	if index < 0 || index >= len(rows) {
		return false
	}
	row := rows[index]
	col := row.start
	if x > gutter {
		textCol := x - gutter
		runes := []rune(lines[row.line])
		used := 0
		for col < row.end {
			w := runewidth.RuneWidth(runes[col])
			if used+w > textCol {
				break
			}
			used += w
			col++
		}
	}
	m.setCursor(row.line, col)
	m.lastCursorRow = -1
	return true
}

// Scroll moves the viewport without moving the cursor.
func (m *Model) Scroll(delta int) {
	if m.width <= 0 || m.height <= 0 || delta == 0 {
		return
	}
	value := m.ta.Value()
	lines := strings.Split(value, "\n")
	if value == "" {
		lines = []string{""}
	}
	gutter := len(strconv.Itoa(max(1, len(lines)))) + 1
	rows := visualRows(lines, max(1, m.width-gutter))
	// Record where the cursor is so the next View keeps this scroll offset
	// instead of snapping back before the first render.
	m.lastCursorRow = visualIndexFor(rows, m.ta.Line(), m.ta.Column())
	maxScroll := max(0, len(rows)-m.height)
	m.scrollRow = min(max(m.scrollRow+delta*3, 0), maxScroll)
}

func (m *Model) setCursor(line, col int) {
	current := m.ta.Line()
	for current < line {
		m.ta.CursorDown()
		next := m.ta.Line()
		if next == current {
			break
		}
		current = next
	}
	for current > line {
		m.ta.CursorUp()
		next := m.ta.Line()
		if next == current {
			break
		}
		current = next
	}
	m.ta.SetCursorColumn(col)
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
	line := m.ta.Line()
	for line < match.line {
		m.ta.CursorDown()
		next := m.ta.Line()
		if next == line {
			break
		}
		line = next
	}
	for line > match.line {
		m.ta.CursorUp()
		next := m.ta.Line()
		if next == line {
			break
		}
		line = next
	}
	m.ta.SetCursorColumn(match.start)
}

// matchOverride reports how a character should be styled: 1 for a match and 2
// for the current match.
func (m *Model) matchOverride(line, col int) int {
	if !m.findOpen || len(m.findMatches) == 0 {
		return 0
	}
	for i, match := range m.findMatches {
		if match.line == line && col >= match.start && col < match.start+match.length {
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
