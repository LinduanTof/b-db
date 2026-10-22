package command

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/i18n"
)

func newOpen() Model {
	m := New(i18n.New("en"))
	m.SetSize(80, 24)
	m.Open()
	return m
}

func typeText(m Model, text string) Model {
	m, _ = m.Update(tea.KeyPressMsg{Text: text})
	return m
}

func TestFilterAndComplete(t *testing.T) {
	m := newOpen()
	m = typeText(m, "exp")
	matches := m.filtered()
	if len(matches) != 1 || matches[0].Name != "export" {
		t.Fatalf("matches = %+v", matches)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.input != "export " {
		t.Fatalf("input after tab = %q", m.input)
	}
}

func TestEnterReturnsSelection(t *testing.T) {
	m := newOpen()
	m = typeText(m, "lang en")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg, ok := cmd().(SelectedMsg)
	if !ok {
		t.Fatalf("message type = %T", cmd())
	}
	if msg.Name != "lang" || len(msg.Args) != 1 || msg.Args[0] != "en" {
		t.Fatalf("message = %+v", msg)
	}
	if m.IsOpen() {
		t.Fatal("palette should close after selection")
	}
}

func TestEnterWithoutInputCompletes(t *testing.T) {
	m := newOpen()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.input != "connect " {
		t.Fatalf("input = %q", m.input)
	}
	if !m.IsOpen() {
		t.Fatal("palette should stay open")
	}
}

func TestEscAndBackspace(t *testing.T) {
	m := newOpen()
	m = typeText(m, "quit")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.input != "qui" {
		t.Fatalf("input = %q", m.input)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the palette")
	}
}

func TestNoMatch(t *testing.T) {
	m := newOpen()
	m = typeText(m, "zzz")
	if got := len(m.filtered()); got != 0 {
		t.Fatalf("matches = %d, want 0", got)
	}
	if m.selectedDef() != nil {
		t.Fatal("selectedDef should be nil")
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter with no match should not run anything")
	}
}
