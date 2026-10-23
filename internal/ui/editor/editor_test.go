package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LinduanTof/b-db/internal/i18n"
)

func TestTokenize(t *testing.T) {
	var block bool
	tokens := tokenizeLine("SELECT COUNT(*) FROM users -- note", &block)
	kinds := map[string]tokenKind{}
	for _, tok := range tokens {
		kinds[tok.text] = tok.kind
	}
	if kinds["SELECT"] != tokKeyword {
		t.Fatalf("SELECT kind = %v", kinds["SELECT"])
	}
	if kinds["COUNT"] != tokFunction {
		t.Fatalf("COUNT kind = %v", kinds["COUNT"])
	}
	foundComment := false
	for _, tok := range tokens {
		if tok.kind == tokComment {
			foundComment = true
		}
	}
	if !foundComment {
		t.Fatal("expected a comment token")
	}

	tokens = tokenizeLine("WHERE name = 'x' AND id = 42 /* c */", &block)
	var hasString, hasNumber, hasComment bool
	for _, tok := range tokens {
		switch tok.kind {
		case tokString:
			hasString = true
		case tokNumber:
			hasNumber = true
		case tokComment:
			hasComment = true
		}
	}
	if !hasString || !hasNumber || !hasComment {
		t.Fatalf("tokens = %+v", tokens)
	}
}

func TestBlockCommentAcrossLines(t *testing.T) {
	var block bool
	tokenizeLine("SELECT /* open", &block)
	if !block {
		t.Fatal("block comment should stay open")
	}
	tokenizeLine("still comment */ SELECT", &block)
	if block {
		t.Fatal("block comment should close")
	}
}

func TestHighlightView(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 8)
	_ = m.Focus()
	m.SetValue("SELECT * FROM users")

	view := m.View()
	if !strings.Contains(view, "SELECT") {
		t.Fatalf("view missing SQL text:\n%s", view)
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("expected ANSI styling in the editor")
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 8 {
		t.Fatalf("lines = %d, want 8", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60", i, w)
		}
	}
}

func TestPlaceholder(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 5)
	_ = m.Focus()
	m.SetValue("")
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "SELECT * FROM ...") {
		t.Fatalf("placeholder missing:\n%s", plain)
	}
}

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
