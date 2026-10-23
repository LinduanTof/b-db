package app

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/db"
)

func TestBuildPageStmt(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		schema  string
		table   string
		offset  int
		size    int
		orderBy []string
		want    string
	}{
		{
			name: "sqlite with primary key", typ: "sqlite", schema: "main", table: "users",
			offset: 4, size: 2, orderBy: []string{"id"},
			want: `SELECT * FROM "users" ORDER BY "id" LIMIT 2 OFFSET 4;`,
		},
		{
			name: "postgres composite key", typ: "postgres", schema: "public", table: "users",
			offset: 0, size: 20, orderBy: []string{"id", "tenant"},
			want: `SELECT * FROM "public"."users" ORDER BY "id", "tenant" LIMIT 20 OFFSET 0;`,
		},
		{
			name: "mysql without order", typ: "mysql", schema: "bdb", table: "users",
			offset: 10, size: 5,
			want: "SELECT * FROM `bdb`.`users` LIMIT 5 OFFSET 10;",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildPageStmt(tc.typ, tc.schema, tc.table, tc.offset, tc.size, tc.orderBy)
			if got != tc.want {
				t.Fatalf("buildPageStmt() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrimaryKeys(t *testing.T) {
	columns := []db.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "tenant", PrimaryKey: true},
		{Name: "name"},
	}
	keys := primaryKeys(columns)
	if len(keys) != 2 || keys[0] != "id" || keys[1] != "tenant" {
		t.Fatalf("primaryKeys() = %#v", keys)
	}
}

func TestTablePagination(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2
	m.editor.SetValue("SELECT 42;")

	m = runCmd(t, m, m.tableData("main", "users"))
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 2 {
		t.Fatalf("page 1 result = %#v, status=%s", rs, m.status)
	}
	if rs.Rows[0][0] != int64(1) {
		t.Fatalf("page 1 first id = %#v", rs.Rows[0][0])
	}
	if m.pager == nil || m.pager.offset != 0 || len(m.pager.orderBy) != 1 || m.pager.orderBy[0] != "id" {
		t.Fatalf("pager = %#v", m.pager)
	}
	if !strings.Contains(m.status, "Page 1") || !strings.Contains(m.status, "2 row(s)") {
		t.Fatalf("status = %q", m.status)
	}
	if got := m.editor.Value(); got != "SELECT 42;" {
		t.Fatalf("table data must not modify the editor, got %q", got)
	}

	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) {
		t.Fatalf("page 2 result = %#v", rs)
	}
	if m.pager.offset != 2 {
		t.Fatalf("page 2 offset = %d", m.pager.offset)
	}
	if !strings.Contains(m.status, "Page 2") {
		t.Fatalf("status=%q", m.status)
	}
	if got := m.editor.Value(); got != "SELECT 42;" {
		t.Fatalf("paging must not modify the editor, got %q", got)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 1 || rs.Rows[0][0] != int64(5) {
		t.Fatalf("page 3 result = %#v", rs)
	}
	if m.pager.offset != 4 || !m.pager.atEnd {
		t.Fatalf("page 3 pager = %#v", m.pager)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	if !m.pager.atEnd || !strings.Contains(m.status, "No more rows") {
		t.Fatalf("end pager = %#v, status=%q", m.pager, m.status)
	}
	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 1 || rs.Rows[0][0] != int64(5) {
		t.Fatalf("results changed after end: %#v", rs)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Text: "p"})
	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) {
		t.Fatalf("previous page result = %#v", rs)
	}
	if m.pager.offset != 2 || m.pager.atEnd {
		t.Fatalf("previous page pager = %#v", m.pager)
	}
}

func TestPaginationClearedByCustomQuery(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2

	m = runCmd(t, m, m.tableData("main", "users"))
	if m.pager == nil {
		t.Fatal("table data should start pagination")
	}
	m.focus = focusResults
	m.editor.SetValue("SELECT 1;")
	m = runCmd(t, m, m.runQuery())
	if m.pager != nil {
		t.Fatal("custom query should clear pagination")
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	if !strings.Contains(m.status, "not a table page") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestPaginationRefreshKeepsPage(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2
	m.editor.SetValue("SELECT 42;")

	m = runCmd(t, m, m.tableData("main", "users"))
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	if m.pager == nil || m.pager.offset != 2 {
		t.Fatalf("pager = %#v", m.pager)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "e"})
	if !m.cellEdit.IsOpen() {
		t.Fatalf("cell editor should open, status=%s", m.status)
	}
	m.cellEdit.SetValue("PageTwo")
	m = sendKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "y"})

	if m.pager == nil || m.pager.offset != 2 {
		t.Fatalf("pager after refresh = %#v", m.pager)
	}
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) || rs.Rows[0][1] != "PageTwo" {
		t.Fatalf("refreshed page = %#v", rs)
	}
	if got := m.editor.Value(); got != "SELECT 42;" {
		t.Fatalf("refresh must not modify the editor, got %q", got)
	}
}

func TestPaginationRefreshFallsBackWhenPageEmpty(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2

	m = runCmd(t, m, m.tableData("main", "users"))
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 1 || rs.Rows[0][0] != int64(5) {
		t.Fatalf("last page = %#v", rs)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Text: "x"})
	if !m.confirm.IsOpen() {
		t.Fatalf("delete confirm should open, status=%s", m.status)
	}
	m = sendKey(t, m, tea.KeyPressMsg{Text: "y"})

	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) || rs.Rows[1][0] != int64(4) {
		t.Fatalf("fallback page = %#v", rs)
	}
	if m.pager == nil || m.pager.offset != 2 {
		t.Fatalf("fallback pager = %#v", m.pager)
	}
	if strings.Contains(m.status, "No more rows") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestPaginationStopsAtFullLastPage(t *testing.T) {
	m := testModel(t, "en")
	m = connect(t, m)
	defer func() {
		if m.conn != nil {
			m.conn.Close()
		}
	}()
	m.cfg.Query.MaxRows = 2
	if _, err := m.conn.Querier().Exec(context.Background(), "DELETE FROM users WHERE id = 5;"); err != nil {
		t.Fatal(err)
	}

	m = runCmd(t, m, m.tableData("main", "users"))
	m.focus = focusResults
	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	rs := m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) {
		t.Fatalf("page 2 = %#v", rs)
	}
	if m.pager.atEnd {
		t.Fatalf("full page should not be marked as the end: %#v", m.pager)
	}

	m = sendKey(t, m, tea.KeyPressMsg{Text: "n"})
	if !m.pager.atEnd || m.pager.offset != 2 {
		t.Fatalf("pager = %#v", m.pager)
	}
	if !strings.Contains(m.status, "No more rows") {
		t.Fatalf("status = %q", m.status)
	}
	rs = m.results.Result()
	if rs == nil || len(rs.Rows) != 2 || rs.Rows[0][0] != int64(3) {
		t.Fatalf("results should stay on the last full page: %#v", rs)
	}
}
