package statusbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCenterRendering(t *testing.T) {
	m := Model{
		Width:  80,
		Left:   "bdb  Results",
		Center: "SQL SELECT * FROM users",
		Right:  "Page 1",
	}
	view := ansi.Strip(m.View())
	if width := ansi.StringWidth(view); width != 80 {
		t.Fatalf("width = %d, want 80", width)
	}
	if !strings.Contains(view, "bdb  Results") || !strings.Contains(view, "SQL SELECT * FROM users") || !strings.Contains(view, "Page 1") {
		t.Fatalf("view missing sections:\n%s", view)
	}
	if !strings.HasSuffix(strings.TrimRight(view, " "), "Page 1") {
		t.Fatalf("right section should stay at the end:\n%s", view)
	}
}

func TestCenterTruncatedWhenNarrow(t *testing.T) {
	m := Model{
		Width:  40,
		Left:   "bdb",
		Center: "SQL SELECT * FROM a_very_long_table_name",
		Right:  "Page 1",
	}
	view := ansi.Strip(m.View())
	if width := ansi.StringWidth(view); width != 40 {
		t.Fatalf("width = %d, want 40", width)
	}
	if !strings.Contains(view, "bdb") || !strings.Contains(view, "SQL") || !strings.HasSuffix(strings.TrimRight(view, " "), "Page 1") {
		t.Fatalf("view sections = %q", view)
	}
}
