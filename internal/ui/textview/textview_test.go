package textview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/i18n"
)

func TestScrollAndClose(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 8)
	m.Open("DDL: users", "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10")
	if !strings.Contains(m.View(), "DDL: users") {
		t.Fatalf("view missing title:\n%s", m.View())
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.scroll != 1 {
		t.Fatalf("scroll = %d, want 1", m.scroll)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m, _ = m.Update(tea.KeyPressMsg{Text: "G"})
	if m.scroll == 0 {
		t.Fatal("G should jump to the bottom")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() {
		t.Fatal("esc should close the viewer")
	}
}
