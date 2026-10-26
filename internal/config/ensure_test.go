package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFileCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dbrix", "config.yaml")
	created, err := EnsureFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected a new config file")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "connections: []") {
		t.Fatalf("template:\n%s", data)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != path {
		t.Fatalf("path = %q, want %q", cfg.Path, path)
	}

	if err := os.WriteFile(path, []byte("custom: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err = EnsureFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("existing config should not be recreated")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "custom: true\n" {
		t.Fatalf("existing config was overwritten: %q", data)
	}
}

func TestResolvePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/dbrix-xdg")
	if got := ResolvePath(""); got != "/tmp/dbrix-xdg/dbrix/config.yaml" {
		t.Fatalf("default path = %q", got)
	}
	if got := ResolvePath("/tmp/custom.yaml"); got != "/tmp/custom.yaml" {
		t.Fatalf("explicit path = %q", got)
	}
}
