package confirm

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/i18n"
)

func TestConfirmAndCancel(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open("Confirm update", "UPDATE users SET name = 'bob' WHERE id = 1;")
	view := m.View()
	if !strings.Contains(view, "Confirm update") || !strings.Contains(view, "UPDATE users") {
		t.Fatalf("view:\n%s", view)
	}

	model, cmd := m.Update(tea.KeyPressMsg{Text: "y"})
	m = model
	if cmd == nil {
		t.Fatal("expected confirm command")
	}
	if _, ok := cmd().(ConfirmedMsg); !ok {
		t.Fatalf("message = %T", cmd())
	}
	if m.IsOpen() {
		t.Fatal("dialog should close after confirm")
	}

	m = New(i18n.New("en"))
	m.SetSize(100, 30)
	m.Open("Confirm", "DELETE")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the dialog")
	}
}
