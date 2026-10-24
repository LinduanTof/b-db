package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := LoadLayoutState(); got != "" {
		t.Fatalf("empty state = %q", got)
	}
	if err := SaveLayoutState(LayoutColumns); err != nil {
		t.Fatal(err)
	}
	if got := LoadLayoutState(); got != LayoutColumns {
		t.Fatalf("state = %q", got)
	}
	info, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "bdb", "layout"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions = %o", perm)
	}
}

func TestNormalizeLayout(t *testing.T) {
	cases := map[string]string{
		"":               LayoutStacked,
		LayoutStacked:    LayoutStacked,
		LayoutColumns:    LayoutColumns,
		LayoutResultsTop: LayoutResultsTop,
		"bogus":          LayoutStacked,
	}
	for input, want := range cases {
		if got := NormalizeLayout(input); got != want {
			t.Fatalf("NormalizeLayout(%q) = %q, want %q", input, got, want)
		}
	}
}
