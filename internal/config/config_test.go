package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadExpandsEnv(t *testing.T) {
	t.Setenv("BDB_TEST_PASSWORD", "s3cret")
	path := writeConfig(t, `
default_connection: pg
connections:
  - name: pg
    type: postgres
    host: localhost
    user: postgres
    password: ${BDB_TEST_PASSWORD}
    database: app
`)
	cfg, warnings, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got := cfg.Connections[0].Password; got != "s3cret" {
		t.Fatalf("password = %q, want s3cret", got)
	}
	if got := cfg.Query.Timeout.Std().String(); got != "30s" {
		t.Fatalf("default timeout = %s, want 30s", got)
	}
	if cfg.Query.MaxRows != 1000 {
		t.Fatalf("default max_rows = %d, want 1000", cfg.Query.MaxRows)
	}
}

func TestLoadMissingEnvFails(t *testing.T) {
	os.Unsetenv("BDB_MISSING_PASSWORD")
	path := writeConfig(t, `
connections:
  - name: pg
    type: postgres
    host: localhost
    user: postgres
    password: ${BDB_MISSING_PASSWORD}
    database: app
`)
	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing environment variable")
	}
	for _, want := range []string{"pg", "password", "BDB_MISSING_PASSWORD"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, warnings, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	if cfg.UI.Language != "auto" {
		t.Fatalf("language = %q, want auto", cfg.UI.Language)
	}
}

func TestValidateDuplicateName(t *testing.T) {
	path := writeConfig(t, `
connections:
  - {name: a, type: sqlite, path: ./a.db}
  - {name: a, type: sqlite, path: ./b.db}
`)
	if _, _, err := Load(path); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestValidateRequiredFields(t *testing.T) {
	path := writeConfig(t, `
connections:
  - {name: pg, type: postgres, host: localhost}
`)
	_, _, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "host, user and database") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFocusModeDefaults(t *testing.T) {
	path := writeConfig(t, "connections: []")
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.FocusMode != FocusModeManual {
		t.Fatalf("focus_mode = %q, want manual", cfg.UI.FocusMode)
	}

	path = writeConfig(t, "ui:\n  focus_mode: auto\nconnections: []")
	cfg, _, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.FocusMode != FocusModeAuto {
		t.Fatalf("focus_mode = %q, want auto", cfg.UI.FocusMode)
	}

	path = writeConfig(t, "ui:\n  focus_mode: nonsense\nconnections: []")
	cfg, _, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.FocusMode != FocusModeManual {
		t.Fatalf("invalid focus_mode should fall back to manual, got %q", cfg.UI.FocusMode)
	}
}

func TestMySQLDatabaseOptional(t *testing.T) {
	cfg := &Config{Connections: []Connection{
		{Name: "m", Type: TypeMySQL, Host: "127.0.0.1", User: "root"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mysql without database should be valid: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	path := writeConfig(t, `
query:
  timeout: 2m
  max_rows: 50
connections: []
`)
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Query.Timeout.Std().String(); got != "2m0s" {
		t.Fatalf("timeout = %s, want 2m0s", got)
	}
}
