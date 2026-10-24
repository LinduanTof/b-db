package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/dialect"
	"github.com/LinduanTof/b-db/internal/i18n"
)

type orderTerm struct {
	column string
	desc   bool
}

type pagination struct {
	schema     string
	table      string
	offset     int
	size       int
	orderBy    []orderTerm
	primary    []string
	sortColumn string
	sortAsc    bool
	stmt       string
	atEnd      bool
	refresh    bool
	metaLoaded bool
	total      int64
	totalKnown bool
}

func (m *Model) runPageQuery(pager *pagination) tea.Cmd {
	if pager == nil {
		return nil
	}
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	m.setContext(contextData)
	page := *pager
	if page.size <= 0 {
		page.size = 1
	}
	page.atEnd = false
	page.stmt = ""
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.querySeq++
	id := m.querySeq
	m.running = true
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Query.Timeout.Std())
	m.cancel = cancel
	m.setStatus(m.t.T(i18n.KeyStatusRunning), false)

	executor := m.executor
	executor.MaxRows = page.size
	conn := m.conn
	typ := conn.Type()
	primary := append([]string(nil), page.primary...)
	needsMeta := !page.metaLoaded
	sortColumn, sortAsc := page.sortColumn, page.sortAsc
	schema, table, offset, size := page.schema, page.table, page.offset, page.size

	return func() tea.Msg {
		if needsMeta {
			if columns, err := conn.Catalog().Columns(ctx, schema, table); err == nil {
				primary = primaryKeys(columns)
				page.metaLoaded = true
			}
		}
		if !page.totalKnown {
			if total, err := tableCount(ctx, conn, typ, schema, table); err == nil {
				page.total = total
				page.totalKnown = true
			}
		}
		page.primary = primary
		page.orderBy = pageOrderTerms(sortColumn, sortAsc, primary)
		page.stmt = buildPageStmt(typ, schema, table, offset, size, page.orderBy)
		out := executor.Run(ctx, page.stmt)
		cancel()
		return tablePageDoneMsg{id: id, outcome: out, page: &page}
	}
}

func buildCountStmt(typ, schema, table string) string {
	return "SELECT COUNT(*) FROM " + dialectFor(typ).Qualified(schema, table) + ";"
}

func tableCount(ctx context.Context, conn db.Conn, typ, schema, table string) (int64, error) {
	result, err := conn.Querier().Query(ctx, buildCountStmt(typ, schema, table), 1)
	if err != nil {
		return 0, err
	}
	if result == nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return 0, fmt.Errorf("count query returned no rows")
	}
	switch value := result.Rows[0][0].(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case []byte:
		return strconv.ParseInt(string(value), 10, 64)
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return strconv.ParseInt(fmt.Sprint(value), 10, 64)
	}
}

func (m *Model) nextPage() tea.Cmd {
	if m.pager == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoPagination), false)
		return nil
	}
	if m.running {
		return nil
	}
	if m.pager.atEnd {
		m.setStatus(m.t.T(i18n.KeyStatusNoMoreRows), false)
		return nil
	}
	if rs := m.results.Result(); rs != nil && len(rs.Rows) < m.pager.size {
		m.pager.atEnd = true
		m.setStatus(m.t.T(i18n.KeyStatusNoMoreRows), false)
		return nil
	}
	page := *m.pager
	page.offset += page.size
	page.atEnd = false
	page.refresh = false
	return m.runPageQuery(&page)
}

func (m *Model) previousPage() tea.Cmd {
	if m.pager == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoPagination), false)
		return nil
	}
	if m.running {
		return nil
	}
	if m.pager.offset == 0 {
		m.setStatus(m.t.T(i18n.KeyStatusFirstPage), false)
		return nil
	}
	page := *m.pager
	page.offset -= page.size
	if page.offset < 0 {
		page.offset = 0
	}
	page.atEnd = false
	page.refresh = false
	return m.runPageQuery(&page)
}

func buildPageStmt(typ, schema, table string, offset, size int, orderBy []orderTerm) string {
	d := dialectFor(typ)
	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(d.Qualified(schema, table))
	if len(orderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, term := range orderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(d.Quote(term.column))
			if term.desc {
				b.WriteString(" DESC")
			}
		}
	}
	fmt.Fprintf(&b, " LIMIT %d OFFSET %d;", size, offset)
	return b.String()
}

func pageOrderTerms(column string, asc bool, primary []string) []orderTerm {
	var terms []orderTerm
	if column != "" {
		terms = append(terms, orderTerm{column: column, desc: !asc})
	}
	for _, key := range primary {
		if key == column {
			continue
		}
		terms = append(terms, orderTerm{column: key})
	}
	return terms
}

func (m *Model) toggleTableSort() tea.Cmd {
	if m.pager == nil {
		return nil
	}
	column := m.results.CursorColumnName()
	if column == "" {
		return nil
	}
	page := *m.pager
	if page.sortColumn == column {
		page.sortAsc = !page.sortAsc
	} else {
		page.sortColumn = column
		page.sortAsc = true
	}
	page.offset = 0
	page.atEnd = false
	page.refresh = false
	return m.runPageQuery(&page)
}

func (m *Model) clearTableSort() tea.Cmd {
	if m.pager == nil || m.pager.sortColumn == "" {
		return nil
	}
	page := *m.pager
	page.sortColumn = ""
	page.sortAsc = true
	page.offset = 0
	page.atEnd = false
	page.refresh = false
	return m.runPageQuery(&page)
}

func primaryKeys(columns []db.Column) []string {
	var keys []string
	for _, column := range columns {
		if column.PrimaryKey {
			keys = append(keys, column.Name)
		}
	}
	return keys
}

func dialectFor(typ string) dialect.Dialect {
	switch typ {
	case "postgres":
		return dialect.Postgres{}
	case "mysql":
		return dialect.MySQL{}
	default:
		return dialect.SQLite{}
	}
}
