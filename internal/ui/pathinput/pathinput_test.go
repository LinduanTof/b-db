package pathinput

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/i18n"
)

func openModel(t *testing.T, value string) Model {
	t.Helper()
	m := New(i18n.New("en"))
	m.SetSize(80, 24)
	m.Open(value)
	return m
}

func TestSubmitValue(t *testing.T) {
	m := openModel(t, "/tmp")
	for _, char := range "/data" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(char)})
	}
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if m.IsOpen() {
		t.Fatal("input should close after submit")
	}
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	msg, ok := cmd().(SubmittedMsg)
	if !ok || msg.Value != "/tmp/data" {
		t.Fatalf("message = %#v", cmd())
	}
}

func TestEscCloses(t *testing.T) {
	m := openModel(t, "/tmp")
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if model.IsOpen() || cmd != nil {
		t.Fatalf("esc: open=%v cmd=%v", model.IsOpen(), cmd)
	}
}

func TestViewLinesShareWidth(t *testing.T) {
	m := openModel(t, "/tmp/exports")
	lines := strings.Split(m.View(), "\n")
	w := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != w {
			t.Fatalf("line %d width = %d, want %d", i, got, w)
		}
	}
}
