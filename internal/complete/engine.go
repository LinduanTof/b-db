package complete

import (
	"sort"
	"strings"
	"sync"
)

type Kind int

const (
	KindKeyword Kind = iota
	KindFunction
	KindTable
	KindColumn
)

func (k Kind) Label() string {
	switch k {
	case KindKeyword:
		return "kw"
	case KindFunction:
		return "fn"
	case KindTable:
		return "tbl"
	case KindColumn:
		return "col"
	default:
		return ""
	}
}

type Candidate struct {
	Text string
	Kind Kind
}

var keywords = []string{
	"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
	"DELETE", "CREATE", "ALTER", "DROP", "TABLE", "INDEX", "VIEW", "JOIN",
	"LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "ON", "GROUP", "BY",
	"ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "ALL", "DISTINCT", "AS",
	"AND", "OR", "NOT", "NULL", "IS", "IN", "EXISTS", "BETWEEN", "LIKE",
	"ILIKE", "CASE", "WHEN", "THEN", "ELSE", "END", "ASC", "DESC", "PRIMARY",
	"KEY", "FOREIGN", "REFERENCES", "DEFAULT", "CONSTRAINT", "UNIQUE", "IF",
	"BEGIN", "COMMIT", "ROLLBACK", "WITH", "RETURNING", "EXPLAIN", "ANALYZE",
	"VACUUM", "PRAGMA", "SHOW", "DESCRIBE", "USE", "DATABASE", "SCHEMA",
	"TRUE", "FALSE", "TRUNCATE", "GRANT", "REVOKE", "TRANSACTION",
}

var functions = []string{
	"COUNT", "SUM", "AVG", "MIN", "MAX", "COALESCE", "NULLIF", "CAST",
	"LOWER", "UPPER", "LENGTH", "SUBSTRING", "ROUND", "ABS", "NOW", "DATE",
	"JSON_EXTRACT", "ARRAY_AGG", "STRING_AGG",
}

type Engine struct {
	mu      sync.RWMutex
	tables  []string
	columns map[string][]string
}

func New() *Engine {
	return &Engine{columns: map[string][]string{}}
}

func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tables = nil
	e.columns = map[string][]string{}
}

func (e *Engine) SetTables(tables []string) {
	seen := map[string]bool{}
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		if table == "" {
			continue
		}
		key := strings.ToLower(table)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, table)
	}
	sort.Strings(out)

	e.mu.Lock()
	e.tables = out
	e.mu.Unlock()
}

func (e *Engine) SetColumns(table string, columns []string) {
	out := append([]string(nil), columns...)
	sort.Strings(out)
	e.mu.Lock()
	e.columns[strings.ToLower(table)] = out
	e.mu.Unlock()
}

func (e *Engine) HasColumns(table string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.columns[strings.ToLower(table)]
	return ok
}

func (e *Engine) Columns(table string) ([]string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cols, ok := e.columns[strings.ToLower(table)]
	if !ok {
		return nil, false
	}
	out := make([]string, len(cols))
	copy(out, cols)
	return out, true
}

func (e *Engine) Suggest(word string) []Candidate {
	qualifier, prefix := SplitWord(word)
	prefixLower := strings.ToLower(prefix)

	e.mu.RLock()
	defer e.mu.RUnlock()

	if qualifier != "" {
		cols, ok := e.columns[strings.ToLower(qualifier)]
		if !ok {
			return nil
		}
		return filterText(cols, prefixLower, KindColumn)
	}

	out := make([]Candidate, 0, 32)
	seen := map[string]bool{}
	add := func(text string, kind Kind) {
		key := strings.ToLower(text)
		if !strings.HasPrefix(key, prefixLower) || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Candidate{Text: text, Kind: kind})
	}
	for _, table := range e.tables {
		add(table, KindTable)
	}
	tables := make([]string, 0, len(e.columns))
	for table := range e.columns {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		for _, column := range e.columns[table] {
			add(column, KindColumn)
		}
	}
	for _, f := range functions {
		add(f, KindFunction)
	}
	for _, k := range keywords {
		add(k, KindKeyword)
	}
	return out
}

func SplitWord(word string) (qualifier, prefix string) {
	if i := strings.LastIndex(word, "."); i >= 0 {
		return word[:i], word[i+1:]
	}
	return "", word
}

func filterText(items []string, prefixLower string, kind Kind) []Candidate {
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item), prefixLower) {
			out = append(out, Candidate{Text: item, Kind: kind})
		}
	}
	return out
}
