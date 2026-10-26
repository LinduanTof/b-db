package complete

import (
	"strings"
	"unicode"
)

// ContextKind is the kind of completion that fits the cursor position.
type ContextKind int

const (
	// ContextAny offers keywords, functions, tables and cached columns.
	ContextAny ContextKind = iota
	// ContextTable expects a table name, for example after FROM or JOIN.
	ContextTable
	// ContextColumn expects an expression, for example after WHERE or AND.
	ContextColumn
)

// TableRef is a table mentioned by the current statement. Alias is filled when
// the statement uses one.
type TableRef struct {
	Schema string
	Table  string
	Alias  string
}

// Context describes what the cursor is completing inside the current
// statement.
type Context struct {
	Kind ContextKind
	// Tables are the tables referenced by the statement's FROM/JOIN/UPDATE/
	// INTO clauses, including tables that appear after the cursor.
	Tables []TableRef
	// Trigger is true when the token before the cursor asks for a new
	// identifier, such as FROM, WHERE, AND or an operator like "=".
	Trigger bool
}

// ResolveTable maps an alias or table name typed as a qualifier to its table
// name. It reports false when the qualifier is not part of the statement.
func ResolveTable(ctx Context, qualifier string) (string, bool) {
	lower := strings.ToLower(qualifier)
	for _, ref := range ctx.Tables {
		if ref.Alias != "" && strings.ToLower(ref.Alias) == lower {
			return ref.Table, true
		}
		if strings.ToLower(ref.Table) == lower {
			return ref.Table, true
		}
		if ref.Schema != "" && strings.ToLower(ref.Schema+"."+ref.Table) == lower {
			return ref.Table, true
		}
	}
	return "", false
}

// Analyze inspects the statement around cursor and reports the completion
// context. cursor is a rune offset into value.
func Analyze(value string, cursor int) Context {
	runes := []rune(value)
	cursor = min(max(cursor, 0), len(runes))
	tokens := tokenizeSQL(runes)
	start, end := statementBounds(tokens, cursor)

	beforeEnd := end
	for i := start; i < end; i++ {
		if tokens[i].start <= cursor && cursor <= tokens[i].end {
			beforeEnd = i
			break
		}
	}
	if beforeEnd < start {
		beforeEnd = start
	}

	kind := ContextAny
	for i := start; i < beforeEnd; i++ {
		tok := tokens[i]
		if tok.kind == tokPunct && tok.text == "(" {
			if kind == ContextTable {
				kind = ContextColumn
			}
			continue
		}
		if tok.kind != tokWord {
			continue
		}
		switch strings.ToUpper(tok.text) {
		case "SELECT", "WHERE", "ON", "HAVING", "SET", "BY", "RETURNING", "USING",
			"AND", "OR", "WHEN", "THEN", "ELSE":
			kind = ContextColumn
		case "FROM", "JOIN", "UPDATE", "INTO", "TRUNCATE":
			kind = ContextTable
		}
	}

	trigger := false
	if beforeEnd > start {
		last := tokens[beforeEnd-1]
		switch last.kind {
		case tokWord:
			trigger = triggerKeywords[strings.ToUpper(last.text)]
		case tokPunct:
			if kind == ContextColumn && columnOperators[last.text] {
				trigger = true
			}
			if kind == ContextTable && last.text == "," {
				trigger = true
			}
		}
	}

	return Context{Kind: kind, Tables: extractTables(tokens[start:end]), Trigger: trigger}
}

var triggerKeywords = map[string]bool{
	"FROM": true, "JOIN": true, "UPDATE": true, "INTO": true,
	"WHERE": true, "AND": true, "OR": true, "ON": true, "HAVING": true,
	"SET": true, "BY": true, "USING": true, "RETURNING": true,
	"LIKE": true, "IN": true, "IS": true, "BETWEEN": true, "NOT": true,
	"EXISTS": true, "WHEN": true, "THEN": true, "ELSE": true,
}

var columnOperators = map[string]bool{
	"=": true, "<": true, ">": true, "<=": true, ">=": true, "<>": true, "!=": true,
}

var clauseKeywords = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true,
	"OFFSET": true, "UNION": true, "EXCEPT": true, "INTERSECT": true,
	"JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true,
	"FULL": true, "CROSS": true, "NATURAL": true, "ON": true, "USING": true,
	"SET": true, "RETURNING": true, "WINDOW": true, "FOR": true,
}

type sqlTokenKind int

const (
	tokWord sqlTokenKind = iota
	tokQuoted
	tokNumber
	tokString
	tokComment
	tokPunct
)

type sqlToken struct {
	text       string
	kind       sqlTokenKind
	start, end int
}

func isNameToken(tok sqlToken) bool {
	if tok.kind == tokQuoted {
		return true
	}
	return tok.kind == tokWord && !clauseKeywords[strings.ToUpper(tok.text)]
}

func tokenizeSQL(runes []rune) []sqlToken {
	tokens := make([]sqlToken, 0, 32)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			start := i
			i += 2
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokComment, start: start, end: i})
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < len(runes) {
				i += 2
			} else {
				i = len(runes)
			}
			tokens = append(tokens, sqlToken{kind: tokComment, start: start, end: i})
		case r == '#':
			start := i
			i++
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokComment, start: start, end: i})
		case r == '\'':
			start := i
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, sqlToken{kind: tokString, start: start, end: i})
		case r == '"' || r == '`':
			quote := r
			start := i
			i++
			var b strings.Builder
			for i < len(runes) {
				if runes[i] == quote {
					if i+1 < len(runes) && runes[i+1] == quote {
						b.WriteRune(quote)
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, sqlToken{text: b.String(), kind: tokQuoted, start: start, end: i})
		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) ||
				runes[i] == '_' || runes[i] == '$') {
				i++
			}
			tokens = append(tokens, sqlToken{text: string(runes[start:i]), kind: tokWord, start: start, end: i})
		case unicode.IsDigit(r):
			start := i
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, sqlToken{text: string(runes[start:i]), kind: tokNumber, start: start, end: i})
		default:
			start := i
			text := string(r)
			if i+1 < len(runes) {
				switch string(runes[i : i+2]) {
				case "<=", ">=", "<>", "!=", "||", "::":
					text = string(runes[i : i+2])
					i++
				}
			}
			i++
			tokens = append(tokens, sqlToken{text: text, kind: tokPunct, start: start, end: i})
		}
	}
	return tokens
}

func statementBounds(tokens []sqlToken, cursor int) (int, int) {
	start := 0
	for i, tok := range tokens {
		if tok.kind == tokPunct && tok.text == ";" && tok.start < cursor {
			start = i + 1
		}
	}
	end := len(tokens)
	for i := start; i < len(tokens); i++ {
		if tokens[i].kind == tokPunct && tokens[i].text == ";" && tokens[i].start >= cursor {
			end = i
			break
		}
	}
	return start, end
}

func extractTables(tokens []sqlToken) []TableRef {
	var refs []TableRef
	seen := map[string]bool{}
	add := func(ref TableRef) {
		if ref.Table == "" {
			return
		}
		key := strings.ToLower(ref.Schema + "." + ref.Table + "." + ref.Alias)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind != tokWord {
			continue
		}
		switch strings.ToUpper(tok.text) {
		case "FROM":
			i = parseTableList(tokens, i+1, true, add) - 1
		case "JOIN", "UPDATE", "INTO":
			i = parseTableList(tokens, i+1, false, add) - 1
		}
	}
	return refs
}

// parseTableList reads one table reference or, when list is true, a
// comma-separated FROM list. It returns the index to continue scanning from.
func parseTableList(tokens []sqlToken, start int, list bool, add func(TableRef)) int {
	i := start
	for i < len(tokens) {
		if tokens[i].kind == tokWord && strings.EqualFold(tokens[i].text, "ONLY") {
			i++
			continue
		}
		if tokens[i].kind == tokPunct && tokens[i].text == "(" {
			depth := 1
			i++
			for i < len(tokens) && depth > 0 {
				if tokens[i].kind == tokPunct && tokens[i].text == "(" {
					depth++
				}
				if tokens[i].kind == tokPunct && tokens[i].text == ")" {
					depth--
				}
				i++
			}
			// Optional alias after a derived table.
			if i < len(tokens) && tokens[i].kind == tokWord && strings.EqualFold(tokens[i].text, "AS") {
				i++
				if i < len(tokens) && isNameToken(tokens[i]) {
					i++
				}
			} else if i < len(tokens) && isNameToken(tokens[i]) {
				i++
			}
			if list && i < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text == "," {
				i++
				continue
			}
			return i
		}
		if !isNameToken(tokens[i]) {
			return i
		}
		ref := TableRef{Table: tokens[i].text}
		i++
		if i+1 < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text == "." &&
			isNameToken(tokens[i+1]) {
			ref.Schema = ref.Table
			ref.Table = tokens[i+1].text
			i += 2
		}
		if i < len(tokens) {
			if tokens[i].kind == tokWord && strings.EqualFold(tokens[i].text, "AS") {
				i++
				if i < len(tokens) && isNameToken(tokens[i]) {
					ref.Alias = tokens[i].text
					i++
				}
			} else if isNameToken(tokens[i]) {
				ref.Alias = tokens[i].text
				i++
			}
		}
		add(ref)
		if !list {
			return i
		}
		if i < len(tokens) && tokens[i].kind == tokPunct && tokens[i].text == "," {
			i++
			continue
		}
		return i
	}
	return i
}
