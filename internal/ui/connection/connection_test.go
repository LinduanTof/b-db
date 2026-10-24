package connection

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/i18n"
)

func newOpen() Model {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open()
	return m
}

func TestDefaultsAndView(t *testing.T) {
	m := newOpen()
	if m.connType != config.TypePostgres {
		t.Fatalf("type = %q", m.connType)
	}
	if got := m.input(fieldPort).Value(); got != "5432" {
		t.Fatalf("port = %q", got)
	}
	view := m.View()
	for _, want := range []string{"New connection", "postgres", "Host", "SSL mode", "Password"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestCycleType(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.connType != config.TypeMySQL {
		t.Fatalf("type = %q", m.connType)
	}
	if got := m.input(fieldPort).Value(); got != "3306" {
		t.Fatalf("port = %q", got)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.connType != config.TypeSQLite {
		t.Fatalf("type = %q", m.connType)
	}
	if len(m.visibleFields()) != 3 {
		t.Fatalf("visible fields = %d, want 3", len(m.visibleFields()))
	}
	view := m.View()
	if strings.Contains(view, "Host") || !strings.Contains(view, "Path") {
		t.Fatalf("sqlite view:\n%s", view)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.connType != config.TypeRedis {
		t.Fatalf("type = %q", m.connType)
	}
	if len(m.visibleFields()) != 5 {
		t.Fatalf("redis visible fields = %d, want 5", len(m.visibleFields()))
	}
	view = m.View()
	if !strings.Contains(view, "Addr") || !strings.Contains(view, "DB") || strings.Contains(view, "Host") {
		t.Fatalf("redis view:\n%s", view)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.connType != config.TypeMongo {
		t.Fatalf("type = %q", m.connType)
	}
	if len(m.visibleFields()) != 3 {
		t.Fatalf("mongo visible fields = %d, want 3", len(m.visibleFields()))
	}
	if !strings.Contains(m.View(), "URI") {
		t.Fatalf("mongo view:\n%s", m.View())
	}
}

func TestMySQLHidesDatabase(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if len(m.visibleFields()) != 6 {
		t.Fatalf("mysql visible fields = %d, want 6", len(m.visibleFields()))
	}
	if strings.Contains(m.View(), "Database") {
		t.Fatalf("mysql form should not ask for a database:\n%s", m.View())
	}
}

func TestArrowKeysMoveFocus(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.focus != 1 {
		t.Fatalf("focus = %d, want 1", m.focus)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.focus != 2 {
		t.Fatalf("focus = %d, want 2", m.focus)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.focus != 1 {
		t.Fatalf("focus = %d, want 1", m.focus)
	}
}

func TestSpaciousRows(t *testing.T) {
	m := newOpen()
	lines := strings.Split(m.View(), "\n")
	if len(lines) < 20 {
		t.Fatalf("expected spacious layout, got %d lines", len(lines))
	}
}

func TestViewAlignedWidth(t *testing.T) {
	m := newOpen()
	lines := strings.Split(m.View(), "\n")
	w := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != w {
			t.Fatalf("line %d width = %d, want %d", i, got, w)
		}
	}
}

func TestPostgresSubmitWithoutDatabase(t *testing.T) {
	m := newOpen()
	m.setValue(fieldName, "local")
	m.setValue(fieldHost, "127.0.0.1")
	m.setValue(fieldUser, "postgres")
	m.setValue(fieldDatabase, "")
	m.focus = len(m.visibleFields()) - 1

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg := cmd().(SubmittedMsg)
	if msg.Conn.Type != config.TypePostgres || msg.Conn.Database != "" {
		t.Fatalf("connection = %+v", msg.Conn)
	}
}

func TestMySQLSubmitWithoutDatabase(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Text: "m"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Text: "u"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Text: "p"})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg := cmd().(SubmittedMsg)
	if msg.Conn.Type != config.TypeMySQL || msg.Conn.Database != "" {
		t.Fatalf("connection = %+v", msg.Conn)
	}
	if msg.Conn.Host != "h" || msg.Conn.User != "u" || msg.Conn.Port != 3306 {
		t.Fatalf("connection = %+v", msg.Conn)
	}
}

func TestSubmitSQLite(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "mydb"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "/tmp/mydb.db"})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg, ok := cmd().(SubmittedMsg)
	if !ok {
		t.Fatalf("message = %T", cmd())
	}
	if msg.Conn.Name != "mydb" || msg.Conn.Type != config.TypeSQLite || msg.Conn.Path != "/tmp/mydb.db" {
		t.Fatalf("connection = %+v", msg.Conn)
	}
	if m.IsOpen() {
		t.Fatal("form should close after submit")
	}
}

func TestValidation(t *testing.T) {
	m := newOpen()
	if _, err := m.build(); err == nil {
		t.Fatal("expected required name error")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "db"})
	if _, err := m.build(); err == nil {
		t.Fatal("expected required host error")
	}

	m.connType = config.TypeMySQL
	m.setValue(fieldHost, "h")
	m.setValue(fieldUser, "u")
	m.setValue(fieldPort, "abc")
	if _, err := m.build(); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestSubmitRedis(t *testing.T) {
	m := newOpen()
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if m.connType != config.TypeRedis {
		t.Fatalf("type = %q", m.connType)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "cache"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "secret"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg := cmd().(SubmittedMsg)
	if msg.Conn.Type != config.TypeRedis || msg.Conn.Addr != "127.0.0.1:6379" ||
		msg.Conn.Password != "secret" || msg.Conn.DB != 2 {
		t.Fatalf("connection = %+v", msg.Conn)
	}
}

func TestSubmitMongo(t *testing.T) {
	m := newOpen()
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if m.connType != config.TypeMongo {
		t.Fatalf("type = %q", m.connType)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Text: "docs"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg := cmd().(SubmittedMsg)
	if msg.Conn.Type != config.TypeMongo || msg.Conn.URI != "mongodb://127.0.0.1:27017" {
		t.Fatalf("connection = %+v", msg.Conn)
	}
}

func TestEscCloses(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the form")
	}
}
