package results

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/linduan-guan/b-db/internal/db"
	"github.com/linduan-guan/b-db/internal/i18n"
)

func sample(rows int) *db.ResultSet {
	rs := &db.ResultSet{
		Columns: []db.ColumnMeta{
			{Name: "id"},
			{Name: "name"},
			{Name: "email_address_column"},
			{Name: "balance"},
			{Name: "note"},
		},
	}
	for i := 0; i < rows; i++ {
		rs.Rows = append(rs.Rows, []any{
			int64(i + 1),
			"张三",
			"user@example.com",
			12.5,
			nil,
		})
	}
	return rs
}

func TestViewDimensionsAndCJK(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 12)
	m.SetResult(sample(50))
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Fatalf("lines = %d, want 12", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60: %q", i, w, line)
		}
	}
	if !strings.Contains(view, "张三") {
		t.Fatalf("view missing CJK content:\n%s", view)
	}
	if !strings.Contains(view, "NULL") {
		t.Fatalf("view missing NULL marker:\n%s", view)
	}
}

func TestCursorNavigationScrolls(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 6)
	m.SetResult(sample(100))

	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.cursorRow != 10 {
		t.Fatalf("cursorRow = %d, want 10", m.cursorRow)
	}
	if m.scrollRow == 0 {
		t.Fatal("scrollRow should move when cursor goes below viewport")
	}

	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if m.cursorCol != 4 {
		t.Fatalf("cursorCol = %d, want 4", m.cursorCol)
	}
	if m.scrollCol == 0 {
		t.Fatal("scrollCol should move when cursor goes beyond visible columns")
	}
	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w != 40 {
			t.Fatalf("line width = %d, want 40: %q", w, line)
		}
	}
}

func TestSortAndColumnWidth(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(80, 10)
	rs := &db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "id"}, {Name: "name"}},
		Rows: [][]any{
			{int64(3), "c"},
			{int64(1), "a"},
			{nil, "none"},
			{int64(2), "b"},
		},
	}
	m.SetResult(rs)

	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	want := []int{1, 3, 0, 2}
	if !sameOrder(m.order, want) {
		t.Fatalf("asc order = %v, want %v", m.order, want)
	}
	if got := m.CursorCell(); got != "1" {
		t.Fatalf("cursor cell = %q, want 1", got)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	want = []int{0, 3, 1, 2}
	if !sameOrder(m.order, want) {
		t.Fatalf("desc order = %v, want %v", m.order, want)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "O"})
	if m.sortCol != -1 || !sameOrder(m.order, []int{0, 1, 2, 3}) {
		t.Fatalf("reset order = %v", m.order)
	}

	m.cursorCol = 1
	before := m.widths[1]
	m, _ = m.Update(tea.KeyPressMsg{Text: "]"})
	if m.widths[1] != before+2 {
		t.Fatalf("width after ] = %d, want %d", m.widths[1], before+2)
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "["})
	if m.widths[1] != before {
		t.Fatalf("width after [ = %d, want %d", m.widths[1], before)
	}
}

func TestCursorCellEmpty(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 5)
	if got := m.CursorCell(); got != "" {
		t.Fatalf("cursor cell = %q, want empty", got)
	}
}

func sameOrder(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestHandleClick(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 12)
	m.SetResult(sample(10))

	if !m.HandleClick(7, 3) {
		t.Fatal("click on a data cell should be handled")
	}
	row, col := m.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", row, col)
	}
	if !m.HandleClick(2, 4) {
		t.Fatal("click on the row number should be handled")
	}
	row, _ = m.CursorPosition()
	if row != 1 {
		t.Fatalf("row = %d, want 1", row)
	}
	if m.HandleClick(7, 1) {
		t.Fatal("header click should not select a cell")
	}
}

func TestCursorTextAndHorizontalScroll(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(50, 10)
	m.SetResult(&db.ResultSet{
		Columns: []db.ColumnMeta{
			{Name: "id"}, {Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "note"},
		},
		Rows: [][]any{{int64(1), "a1", "b1", "c1", "line1\nline2"}},
	})

	m.cursorCol = 4
	if got := m.CursorText(); got != "line1\nline2" {
		t.Fatalf("cursor text = %q", got)
	}
	if got := m.CursorColumnName(); got != "note" {
		t.Fatalf("column = %q", got)
	}

	m.cursorCol = 0
	m.ScrollHorizontal(1)
	first := m.scrollCol
	if first == 0 {
		t.Fatal("horizontal scroll should move the viewport")
	}
	if m.cursorCol != first {
		t.Fatalf("cursorCol = %d, scrollCol = %d", m.cursorCol, m.scrollCol)
	}
	m.ScrollHorizontal(-1)
	if m.scrollCol >= first {
		t.Fatalf("left scroll did not move back: %d -> %d", first, m.scrollCol)
	}
}

func TestScroll(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 12)
	m.SetResult(sample(20))
	m.Scroll(1)
	if row, _ := m.CursorPosition(); row != 3 {
		t.Fatalf("row = %d, want 3", row)
	}
	m.Scroll(-1)
	if row, _ := m.CursorPosition(); row != 0 {
		t.Fatalf("row = %d, want 0", row)
	}
}

func TestBorders(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(60, 12)
	m.SetResult(sample(5))
	view := m.View()
	for _, want := range []string{"┌", "┬", "┐", "├", "┼", "┤", "└", "┴", "┘", "│"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing border %q:\n%s", want, view)
		}
	}
}

func TestNoResultPromptAndExec(t *testing.T) {
	m := New(i18n.New("en"))
	m.SetSize(40, 5)
	if got := m.View(); !strings.Contains(got, "Run a query") {
		t.Fatalf("prompt = %q", got)
	}
	m.SetExec(3, 0)
	if got := m.View(); !strings.Contains(got, "3 row(s) affected") {
		t.Fatalf("exec view = %q", got)
	}
	m.SetError("boom")
	if got := m.View(); !strings.Contains(got, "boom") {
		t.Fatalf("error view = %q", got)
	}
}
