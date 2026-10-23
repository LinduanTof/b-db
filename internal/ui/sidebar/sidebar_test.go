package sidebar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/i18n"
)

func TestHandleClickConnects(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)

	cmd := m.HandleClick(0)
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
