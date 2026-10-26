package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportDirStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, ok := LoadExportDirState(); ok {
		t.Fatal("unexpected state file")
	}
	if err := SaveExportDirState("/tmp/exports"); err != nil {
		t.Fatal(err)
	}
	if got, ok := LoadExportDirState(); !ok || got != "/tmp/exports" {
		t.Fatalf("state = %q ok=%v", got, ok)
	}
	info, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "dbrix", "export_dir"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions = %o", perm)
	}
}

func TestExportDirStateCleared(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := SaveExportDirState(""); err != nil {
		t.Fatal(err)
	}
	if got, ok := LoadExportDirState(); !ok || got != "" {
		t.Fatalf("state = %q ok=%v", got, ok)
	}
}
