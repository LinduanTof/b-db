package sidebar

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/i18n"
)

type fakeConn struct {
	db.Conn
	name string
	typ  string
}

func (f fakeConn) Name() string { return f.name }
func (f fakeConn) Type() string { return f.typ }

func TestPostgresDatabaseTree(t *testing.T) {
	m := New([]config.Connection{
		{Name: "pg", Type: config.TypePostgres, Database: "bdb", Host: "h", User: "u"},
	}, i18n.New("en"))
	m.SetSize(40, 12)
	m.SetDatabases("pg", []DatabaseEntry{
		{Name: "bdb", Session: "pg"},
		{Name: "postgres", Session: "pg/postgres"},
	})
	m.SetActiveSessions([]string{"pg", "pg/postgres"})
	if cmd := m.SetDatabaseConn(fakeConn{name: "pg", typ: config.TypePostgres}, "pg", "bdb", "pg"); cmd == nil {
		t.Fatal("selecting a database should load its schemas")
	}

	view := ansi.Strip(m.View())
	for _, want := range []string{"▾ ● ▤ bdb", "▸ ◉ ▤ postgres", "2 active"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestHandleClickConnects(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)

	if cmd := m.HandleClick(0); cmd != nil {
		t.Fatal("clicking the header should do nothing")
	}
	cmd := m.HandleClick(1)
	if cmd == nil {
		t.Fatal("click on a connection should activate it")
	}
	msg, ok := cmd().(ConnectMsg)
	if !ok || msg.Name != "local" {
		t.Fatalf("message = %#v", cmd())
	}
}

func TestConnectionIconsAndAlignment(t *testing.T) {
	m := New([]config.Connection{
		{Name: "a", Type: config.TypeSQLite, Path: "a.db"},
		{Name: "long-connection", Type: config.TypePostgres, Host: "h", User: "u", Database: "d"},
	}, i18n.New("en"))
	m.SetSize(40, 10)

	view := ansi.Strip(m.View())
	lines := strings.Split(view, "\n")
	sqliteCol, postgresCol := -1, -1
	for _, line := range lines {
		if idx := strings.Index(line, "sqlite"); idx >= 0 {
			sqliteCol = ansi.StringWidth(line[:idx])
		}
		if idx := strings.Index(line, "postgres"); idx >= 0 {
			postgresCol = ansi.StringWidth(line[:idx])
		}
	}
	if sqliteCol < 0 || postgresCol < 0 {
		t.Fatalf("types not rendered:\n%s", view)
	}
	if sqliteCol != postgresCol {
		t.Fatalf("type columns: sqlite=%d postgres=%d\n%s", sqliteCol, postgresCol, view)
	}
	if !strings.Contains(view, typeIcon(config.TypeSQLite)) ||
		!strings.Contains(view, typeIcon(config.TypePostgres)) {
		t.Fatalf("icons missing:\n%s", view)
	}
}

func TestActiveSessionsInSidebar(t *testing.T) {
	m := New([]config.Connection{
		{Name: "a", Type: config.TypeSQLite, Path: "a.db"},
		{Name: "b", Type: config.TypePostgres, Host: "h", User: "u", Database: "d"},
		{Name: "c", Type: config.TypeMySQL, Host: "h", User: "u", Database: "d"},
	}, i18n.New("en"))
	m.SetSize(40, 10)
	m.connName = "a"
	m.SetActiveSessions([]string{"a", "b"})

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "2 active") {
		t.Fatalf("session count missing:\n%s", view)
	}
	if !strings.Contains(view, "●") || !strings.Contains(view, "◉") || !strings.Contains(view, "○") {
		t.Fatalf("session markers missing:\n%s", view)
	}
}

func TestTableSingleClickSelectsDoubleClickOpens(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)
	table := &Node{Kind: KindTable, Label: "users", Sub: "table", ConnName: "local", Schema: "main", Table: "users"}
	schema := &Node{Kind: KindSchema, Label: "main", ConnName: "local", Schema: "main", Expanded: true, Children: []*Node{table}}
	m.roots[0].Expanded = true
	m.roots[0].Children = []*Node{schema}
	m.rebuild()

	if cmd := m.HandleClick(3); cmd != nil {
		t.Fatal("single click on a table should only select it")
	}
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want table row", m.cursor)
	}
	cmd := m.HandleClick(3)
	if cmd == nil {
		t.Fatal("double click on a table should open it")
	}
	msg, ok := cmd().(TableActivateMsg)
	if !ok || msg.Schema != "main" || msg.Table != "users" {
		t.Fatalf("message = %#v", cmd())
	}
}

func TestMySQLDatabaseTree(t *testing.T) {
	m := New([]config.Connection{
		{Name: "my", Type: config.TypeMySQL, Host: "h", User: "u"},
	}, i18n.New("en"))
	m.SetSize(40, 12)
	m.SetDatabases("my", []DatabaseEntry{
		{Name: "app", Session: "my/app"},
		{Name: "report", Session: "my/report"},
	})
	if cmd := m.SetDatabaseConn(fakeConn{name: "my/app", typ: config.TypeMySQL}, "my", "app", "my/app"); cmd == nil {
		t.Fatal("selecting a MySQL database should load its tables")
	}
	var database *Node
	for _, node := range m.roots[0].Children {
		if node.Kind == KindDatabase && node.Label == "app" {
			database = node
		}
	}
	if database == nil || database.Schema != "app" || !database.Expanded {
		t.Fatalf("database node = %#v", database)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "▾ ● ▤ app") || !strings.Contains(view, "▸ ○ ▤ report") {
		t.Fatalf("view missing database tree:\n%s", view)
	}
}

func TestNoConnectionsShowsConfigPath(t *testing.T) {
	m := New(nil, i18n.New("en"))
	m.SetSize(80, 10)
	m.SetConfigPath("/tmp/bdb/config.yaml")
	if view := m.View(); !strings.Contains(view, "/tmp/bdb/config.yaml") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestHandleClickOutOfRange(t *testing.T) {
	m := New(nil, i18n.New("en"))
	m.SetSize(30, 10)
	if cmd := m.HandleClick(5); cmd != nil {
		t.Fatal("click outside the list should do nothing")
	}
	if cmd := m.HandleClick(-1); cmd != nil {
		t.Fatal("negative row should do nothing")
	}
}

func TestStaleLoadResultIsIgnored(t *testing.T) {
	m := New(nil, i18n.New("en"))
	m.epoch = 2
	node := &Node{Kind: KindConnection, Loading: true}
	m.Update(SchemasLoadedMsg{Epoch: 1, Node: node, Schemas: []string{"old"}})
	if !node.Loading || len(node.Children) != 0 {
		t.Fatalf("stale message mutated node: %+v", node)
	}
	m.Update(SchemasLoadedMsg{Epoch: 2, Node: node, Schemas: []string{"current"}})
	if node.Loading || len(node.Children) != 1 {
		t.Fatalf("current message not applied: %+v", node)
	}
}

func TestTableKeys(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)
	table := &Node{Kind: KindTable, Label: "users", Sub: "table", ConnName: "local", Schema: "main", Table: "users"}
	schema := &Node{Kind: KindSchema, Label: "main", ConnName: "local", Schema: "main", Expanded: true, Children: []*Node{table}}
	m.roots[0].Expanded = true
	m.roots[0].Children = []*Node{schema}
	m.rebuild()
	m.cursor = 2
	m.rebuild()

	check := func(key tea.KeyPressMsg, want func(tea.Msg) bool) {
		t.Helper()
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("%q did not produce a command", key.String())
		}
		if msg := cmd(); !want(msg) {
			t.Fatalf("%q produced %#v", key.String(), msg)
		}
	}

	check(tea.KeyPressMsg{Code: 's'}, func(msg tea.Msg) bool { _, ok := msg.(TableOpenMsg); return ok })
	check(tea.KeyPressMsg{Code: 'm'}, func(msg tea.Msg) bool { _, ok := msg.(TableMenuMsg); return ok })
	check(tea.KeyPressMsg{Code: 'o'}, func(msg tea.Msg) bool { _, ok := msg.(EditorOpenMsg); return ok })
	check(tea.KeyPressMsg{Code: tea.KeyEnter}, func(msg tea.Msg) bool { _, ok := msg.(TableActivateMsg); return ok })
}

func TestCursorPathRestore(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)
	table := &Node{Kind: KindTable, Label: "users", Sub: "table", ConnName: "local", Schema: "main", Table: "users"}
	schema := &Node{Kind: KindSchema, Label: "main", ConnName: "local", Schema: "main", Expanded: true, Children: []*Node{table}}
	m.roots[0].Expanded = true
	m.roots[0].Children = []*Node{schema}
	m.rebuild()
	m.cursor = 2

	path := m.CursorPath()
	if path == "" {
		t.Fatal("empty cursor path")
	}
	m.cursor = 0
	m.RestoreCursor(path)
	m.rebuild()
	n := m.selected()
	if n == nil || n.Kind != KindTable || n.Table != "users" {
		t.Fatalf("restored node = %#v", n)
	}
}

func TestCursorPathWaitsForAsyncChildren(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)
	table := &Node{Kind: KindTable, Label: "users", Sub: "table", ConnName: "local", Schema: "main", Table: "users"}
	path := nodePath(table)

	m.roots[0].Expanded = true
	m.RestoreCursor(path)
	m.rebuild()
	if m.pendingCursor == "" {
		t.Fatal("pending path should survive a rebuild without the node")
	}

	schema := &Node{Kind: KindSchema, Label: "main", ConnName: "local", Schema: "main", Expanded: true, Children: []*Node{table}}
	m.roots[0].Children = []*Node{schema}
	m.rebuild()
	if n := m.selected(); n == nil || n.Table != "users" {
		t.Fatalf("node after async load = %#v", n)
	}
	if m.pendingCursor != "" {
		t.Fatalf("pending path = %q, want cleared", m.pendingCursor)
	}
}
