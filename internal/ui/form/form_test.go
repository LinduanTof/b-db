package form

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/i18n"
)

func testForm(t *testing.T) Model {
	t.Helper()
	m := New(i18n.New("en"))
	m.SetSize(80, 24)
	m.Open("test", "Title", "note", []Field{
		{Label: "name", Value: "alice"},
		{Label: "kind", Choices: []string{"A", "B"}},
	})
	return m
}

func TestFormSubmit(t *testing.T) {
	m := testForm(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.fields[1].ValueIndex != 1 {
		t.Fatalf("choice = %d, want 1", m.fields[1].ValueIndex)
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.IsOpen() {
		t.Fatal("form should close after submit")
	}
	if cmd == nil {
		t.Fatal("submit should return a command")
	}
	msg, ok := cmd().(SubmittedMsg)
	if !ok {
		t.Fatalf("unexpected message %#v", cmd())
	}
	if msg.ID != "test" || msg.Values[0] != "alice" || msg.Values[1] != "B" || msg.Choices[1] != 1 {
		t.Fatalf("submitted %#v", msg)
	}
}

func TestFormTextInput(t *testing.T) {
	m := testForm(t)
	m, _ = m.Update(tea.KeyPressMsg{Text: "!", Code: '!'})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(SubmittedMsg)
	if msg.Values[0] != "alicex" {
		t.Fatalf("value = %q, want alicex", msg.Values[0])
	}
}

func TestFormEsc(t *testing.T) {
	m := testForm(t)
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() || cmd != nil {
		t.Fatal("esc should close without a command")
	}
}

func TestFormViewWidth(t *testing.T) {
	m := testForm(t)
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 5 {
		t.Fatalf("view too short:\n%s", view)
	}
	width := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}
	if !strings.Contains(ansi.Strip(view), "▸ name") {
		t.Fatalf("active field marker missing:\n%s", ansi.Strip(view))
	}
}

func TestFormErrorReopens(t *testing.T) {
	m := testForm(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m.SetError("bad value")
	if !m.IsOpen() {
		t.Fatal("SetError should reopen the form")
	}
	if !strings.Contains(ansi.Strip(m.View()), "bad value") {
		t.Fatal("error text missing from the view")
	}
}
