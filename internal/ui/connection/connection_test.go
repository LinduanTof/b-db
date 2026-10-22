package connection

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/i18n"
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
	m.setValue(fieldDatabase, "d")
	m.setValue(fieldPort, "abc")
	if _, err := m.build(); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestEscCloses(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the form")
	}
}
