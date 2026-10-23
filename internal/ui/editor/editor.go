package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
}

func New(t *i18n.Translator) Model {
	ta := textarea.New()
	ta.Prompt = "› "
	ta.Placeholder = "SELECT * FROM ..."
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	return Model{t: t, ta: ta}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.ta.SetWidth(max(width, 1))
	m.ta.SetHeight(max(height, 1))
}

func (m *Model) Focus() tea.Cmd { return m.ta.Focus() }

func (m *Model) Blur() { m.ta.Blur() }

func (m Model) Value() string { return m.ta.Value() }

func (m *Model) SetValue(s string) {
	m.ta.SetValue(s)
	m.ta.CursorEnd()
	m.scrollRow = 0
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
	if cursorLine < m.scrollRow {
		m.scrollRow = cursorLine
	}
	if cursorLine >= m.scrollRow+m.height {
		m.scrollRow = cursorLine - m.height + 1
	}
	if m.scrollRow < 0 {
		m.scrollRow = 0
	}

	gutter := len(strconv.Itoa(max(1, len(lines)))) + 1
	showCursor := m.ta.Focused()
	out := make([]string, 0, m.height)
	inBlock := false
	for i, line := range lines {
		tokens := tokenizeLine(line, &inBlock)
		if i < m.scrollRow {
			continue
		}
		if len(out) >= m.height {
			break
		}
		if value == "" {
			out = append(out, m.placeholderLine(gutter, showCursor))
			continue
		}
		out = append(out, m.renderLine(tokens, i, cursorLine, cursorCol, gutter, showCursor))
	}
	for len(out) < m.height {
		out = append(out, "")
	}
	return textutil.FitLines(strings.Join(out, "\n"), m.width, m.height)
}

func (m *Model) placeholderLine(gutter int, showCursor bool) string {
	line := theme.Muted.Render(fmt.Sprintf("%*d ", gutter-1, 1)) +
		theme.Muted.Render(m.ta.Placeholder)
	if showCursor {
		line += theme.CellCursor.Render(" ")
	}
	return textutil.FitLine(line, m.width)
}

func (m *Model) renderLine(tokens []token, lineIndex, cursorLine, cursorCol, gutter int, showCursor bool) string {
	var b strings.Builder
	b.WriteString(theme.Muted.Render(fmt.Sprintf("%*d ", gutter-1, lineIndex+1)))

	runeOffset := 0
	cursorPlaced := false
	for _, tok := range tokens {
		style := styleFor(tok.kind)
		tokRunes := []rune(tok.text)
		if showCursor && lineIndex == cursorLine && cursorCol >= runeOffset && cursorCol < runeOffset+len(tokRunes) {
			index := cursorCol - runeOffset
			if index > 0 {
				b.WriteString(style.Render(string(tokRunes[:index])))
			}
			b.WriteString(theme.CellCursor.Render(string(tokRunes[index])))
			if index+1 < len(tokRunes) {
				b.WriteString(style.Render(string(tokRunes[index+1:])))
			}
			cursorPlaced = true
		} else {
			b.WriteString(style.Render(tok.text))
		}
		runeOffset += len(tokRunes)
	}
	if showCursor && lineIndex == cursorLine && !cursorPlaced {
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
