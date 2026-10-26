package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := LoadThemeState(); got != "" {
		t.Fatalf("empty state = %q", got)
	}
	if err := SaveThemeState("dracula"); err != nil {
		t.Fatal(err)
	}
	if got := LoadThemeState(); got != "dracula" {
		t.Fatalf("state = %q", got)
	}
	info, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "dbrix", "theme"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}
}
