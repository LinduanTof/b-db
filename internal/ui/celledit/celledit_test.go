package celledit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/i18n"
)

func TestSubmit(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open("Edit users.name", "alice")
	if view := m.View(); !strings.Contains(view, "Edit users.name") || !strings.Contains(view, "alice") {
		t.Fatalf("view:\n%s", view)
	}

	m.SetValue("bob")
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg, ok := cmd().(SubmittedMsg)
	if !ok || msg.Value != "bob" {
		t.Fatalf("message = %#v", cmd())
	}
	if m.IsOpen() {
		t.Fatal("editor should close after submit")
	}
}

func TestCancel(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open("Edit t.c", "x")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the editor")
	}
}
