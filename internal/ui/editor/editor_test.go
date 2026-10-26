package editor

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

func TestFindCyclesMatches(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 8)
	m.SetValue("select id from users\nselect name from users")
	m.OpenFind()
	for _, r := range "select" {
		m, _ = m.UpdateFind(tea.KeyPressMsg{Text: string(r), Code: r})
	}
	query, current, total := m.FindStatus()
	if query != "select" || current != 1 || total != 2 {
		t.Fatalf("find status = %q %d/%d, want select 1/2", query, current, total)
	}
	m, _ = m.UpdateFind(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, current, _ = m.FindStatus(); current != 2 {
		t.Fatalf("current = %d, want 2", current)
	}
	m, _ = m.UpdateFind(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, current, _ = m.FindStatus(); current != 1 {
		t.Fatalf("current = %d, want wrap to 1", current)
	}
	m, _ = m.UpdateFind(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.Finding() {
		t.Fatal("esc should close the find prompt")
	}
}

func TestFindNoMatchAndBackspace(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 8)
	m.SetValue("select 1")
	m.OpenFind()
	for _, r := range "zzz" {
		m, _ = m.UpdateFind(tea.KeyPressMsg{Text: string(r), Code: r})
	}
	if _, _, total := m.FindStatus(); total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	m, _ = m.UpdateFind(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if query, _, _ := m.FindStatus(); query != "zz" {
		t.Fatalf("query = %q, want zz", query)
	}
}

func TestViewWrapsLongLines(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(30, 6)
	m.SetValue("SELECT " + strings.Repeat("x", 60))
	m.Focus()
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 6 {
		t.Fatalf("lines = %d, want 6", len(lines))
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 30 {
			t.Fatalf("line %d width = %d, want 30: %q", i, got, line)
		}
	}
	plain := ansi.Strip(view)
	if strings.Count(plain, "x") < 50 {
		t.Fatalf("wrapped text was truncated:\n%s", plain)
	}
	if !strings.Contains(strings.Split(plain, "\n")[0], "SELECT") {
		t.Fatalf("first visual row should start with SELECT:\n%s", plain)
	}
	// Continuation rows must not repeat the line number.
	if strings.HasPrefix(strings.Split(plain, "\n")[1], " 2 ") {
		t.Fatalf("continuation row shows a line number:\n%s", plain)
	}
}

func TestViewWrapsOnCJKWidth(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(20, 6)
	m.SetValue(strings.Repeat("中", 20))
	m.Focus()
	for i, line := range strings.Split(m.View(), "\n") {
		if got := ansi.StringWidth(line); got != 20 {
			t.Fatalf("line %d width = %d, want 20", i, got)
		}
	}
}

func TestVisualRowsMapping(t *testing.T) {
	rows := visualRows([]string{"abcdefghij"}, 4)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3", rows)
	}
	if got := visualIndexFor(rows, 0, 0); got != 0 {
		t.Fatalf("col 0 -> %d, want 0", got)
	}
	if got := visualIndexFor(rows, 0, 4); got != 1 {
		t.Fatalf("col 4 -> %d, want 1", got)
	}
	if got := visualIndexFor(rows, 0, 10); got != 2 {
		t.Fatalf("end of line -> %d, want 2", got)
	}
}

func TestViewScrollsToWrappedCursor(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(24, 3)
	m.SetValue("short\n" + strings.Repeat("y", 80))
	m.Focus()
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "y") {
		t.Fatalf("cursor row should be visible after wrapping:\n%s", plain)
	}
}

func TestHandleClickPositionsCursor(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 6)
	m.SetValue("first\nsecond line")
	m.Focus()
	// gutter is 2 columns for a two-line document.
	if ok := m.HandleClick(2+3, 1); !ok {
		t.Fatal("click should land inside the editor")
	}
	if line, col := m.ta.Line(), m.ta.Column(); line != 1 || col != 3 {
		t.Fatalf("cursor = %d:%d, want 1:3", line, col)
	}
	if ok := m.HandleClick(0, 0); !ok {
		t.Fatal("click on the gutter should still move the cursor")
	}
	if line, col := m.ta.Line(), m.ta.Column(); line != 0 || col != 0 {
		t.Fatalf("cursor = %d:%d, want 0:0", line, col)
	}
}

func TestHandleClickOnWrappedRow(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(20, 5)
	m.SetValue(strings.Repeat("a", 40))
	m.Focus()
	// Text width is 18, so row 1 starts at column 18.
	if ok := m.HandleClick(2, 1); !ok {
		t.Fatal("click should land on the wrapped row")
	}
	if line, col := m.ta.Line(), m.ta.Column(); line != 0 || col != 18 {
		t.Fatalf("cursor = %d:%d, want 0:18", line, col)
	}
}

func TestScrollMovesViewportOnly(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(20, 3)
	m.SetValue("1\n2\n3\n4\n5\n6\n7\n8")
	m.Focus()
	_ = m.View()
	if m.scrollRow != 5 {
		t.Fatalf("scrollRow = %d, want 5", m.scrollRow)
	}
	m.Scroll(-1)
	if m.scrollRow != 2 {
		t.Fatalf("scrollRow = %d, want 2", m.scrollRow)
	}
	if m.ta.Line() != 7 {
		t.Fatalf("scroll must not move the cursor, line = %d", m.ta.Line())
	}
	// Re-running View must not snap back to the cursor.
	view := m.View()
	if m.scrollRow != 2 {
		t.Fatalf("View snapped back to %d", m.scrollRow)
	}
	if !strings.Contains(ansi.Strip(view), "3") {
		t.Fatalf("unexpected view after scrolling:\n%s", ansi.Strip(view))
	}
}

func TestHandleClickBelowWrappedLine(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(20, 8)
	m.SetValue(strings.Repeat("a", 40) + "\nsecond")
	m.Focus()
	m.ta.MoveToBegin()
	if ok := m.HandleClick(2, 3); !ok {
		t.Fatal("click should land in the editor")
	}
	if line := m.ta.Line(); line != 1 {
		t.Fatalf("click landed on line %d, want 1", line)
	}
}

func TestFindBelowWrappedLine(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(20, 8)
	m.SetValue(strings.Repeat("a", 40) + "\nsecond target")
	m.Focus()
	m.ta.MoveToBegin()
	m.OpenFind()
	for _, r := range "target" {
		m, _ = m.UpdateFind(tea.KeyPressMsg{Text: string(r), Code: r})
	}
	if line := m.ta.Line(); line != 1 {
		t.Fatalf("find landed on line %d, want 1", line)
	}
}

func TestLayoutCacheReflectsEdits(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 6)
	m.SetValue("alpha\nbeta\ngamma")
	m.Focus()
	if view := ansi.Strip(m.View()); !strings.Contains(view, "alpha") {
		t.Fatalf("missing initial text:\n%s", view)
	}
	m.SetValue("alpha\nBETA\ngamma")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "BETA") || strings.Contains(view, "beta") {
		t.Fatalf("cache did not update after edit:\n%s", view)
	}
	// A block comment change must propagate to the following lines.
	m.SetValue("alpha\n/* start\nBETA\ngamma")
	m.SetValue("alpha\nBETA\ngamma */")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "BETA") {
		t.Fatalf("cache did not update after removing the comment opener:\n%s", view)
	}
}

func TestLayoutCacheHandlesResize(t *testing.T) {
	m := New(i18n.New("en"))
	value := "SELECT " + strings.Repeat("x", 50)
	m.SetValue(value)
	m.Focus()
	for _, width := range []int{60, 30, 20, 40} {
		m.SetSize(width, 6)
		for i, line := range strings.Split(m.View(), "\n") {
			if got := ansi.StringWidth(line); got != width {
				t.Fatalf("width %d line %d = %d", width, i, got)
			}
		}
	}
}
