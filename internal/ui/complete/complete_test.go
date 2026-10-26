package complete

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	ac "github.com/LinduanTof/dbrix/internal/complete"
	"github.com/LinduanTof/dbrix/internal/i18n"
)

func TestViewAndAccept(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(80, 24)
	m.Open("sel", []ac.Candidate{
		{Text: "SELECT", Kind: ac.KindKeyword},
		{Text: "SELECTED", Kind: ac.KindKeyword},
	})
	view := m.View()
	if !contains(view, "[kw] SELECT") || !contains(view, "Completions: sel") {
		t.Fatalf("view:\n%s", view)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model
	if cmd == nil {
		t.Fatal("expected accept command")
	}
	msg, ok := cmd().(AcceptedMsg)
	if !ok {
		t.Fatalf("message = %T", cmd())
	}
	if msg.Text != "SELECTED" || msg.PrefixLen != 3 {
		t.Fatalf("accepted = %+v", msg)
	}
	if m.IsOpen() {
		t.Fatal("panel should close after accept")
	}
}

func TestEscAndClamp(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(80, 24)
	m.Open("x", []ac.Candidate{{Text: "x", Kind: ac.KindColumn}})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 0 {
		t.Fatalf("selected = %d", m.selected)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
