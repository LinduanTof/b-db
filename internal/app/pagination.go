package app

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/dialect"
	"github.com/LinduanTof/b-db/internal/i18n"
)

type pagination struct {
	schema     string
	table      string
	offset     int
	size       int
	orderBy    []string
	stmt       string
	atEnd      bool
	refresh    bool
	metaLoaded bool
}

func (m *Model) runPageQuery(pager *pagination) tea.Cmd {
	if pager == nil {
		return nil
	}
	if m.conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
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
	orderBy := append([]string(nil), page.orderBy...)
	needsMeta := !page.metaLoaded
	schema, table, offset, size := page.schema, page.table, page.offset, page.size

	return func() tea.Msg {
		if needsMeta {
			if columns, err := conn.Catalog().Columns(ctx, schema, table); err == nil {
				orderBy = primaryKeys(columns)
				page.metaLoaded = true
			}
		}
		page.orderBy = orderBy
		page.stmt = buildPageStmt(typ, schema, table, offset, size, orderBy)
		out := executor.Run(ctx, page.stmt)
		cancel()
		return tablePageDoneMsg{id: id, outcome: out, page: &page}
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

func buildPageStmt(typ, schema, table string, offset, size int, orderBy []string) string {
	d := dialectFor(typ)
	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(d.Qualified(schema, table))
	if len(orderBy) > 0 {
		b.WriteString(" ORDER BY ")
		for i, column := range orderBy {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(d.Quote(column))
		}
	}
	fmt.Fprintf(&b, " LIMIT %d OFFSET %d;", size, offset)
	return b.String()
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
