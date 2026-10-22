package editor

import (
	"testing"

	"github.com/linduan/bdb/internal/i18n"
)

func TestWordBeforeCursor(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 5)
	m.SetValue("SELECT * FROM users WHERE users.na")
	word, n := m.WordBeforeCursor()
	if word != "users.na" || n != 8 {
		t.Fatalf("word = %q, n = %d", word, n)
	}
}

func TestReplaceBeforeCursor(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 5)
	_ = m.Focus()
	m.SetValue("SELECT * FROM users WHERE users.na")
	m.ReplaceBeforeCursor(2, "name")
	if got := m.Value(); got != "SELECT * FROM users WHERE users.name" {
		t.Fatalf("value = %q", got)
	}
}

func TestWordAtLineStart(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 5)
	m.SetValue("select")
	word, n := m.WordBeforeCursor()
	if word != "select" || n != 6 {
		t.Fatalf("word = %q, n = %d", word, n)
	}
}
