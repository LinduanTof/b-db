package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeConnectionsRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	conn := Connection{Name: "local", Type: TypeSQLite, Path: "./demo.db"}
	if err := AppendRuntimeConnection(conn); err != nil {
		t.Fatal(err)
	}
	if err := AppendRuntimeConnection(conn); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRuntimeConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name != "local" || loaded[0].Path != "./demo.db" {
		t.Fatalf("loaded = %+v", loaded)
	}

	info, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "bdb", "connections.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestMergeRuntime(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := AppendRuntimeConnection(Connection{Name: "added", Type: TypeSQLite, Path: "./x.db"}); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Connections: []Connection{{Name: "base", Type: TypeSQLite, Path: "./base.db"}}}
	if err := MergeRuntime(cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 2 || cfg.Find("added") == nil {
		t.Fatalf("connections = %+v", cfg.Connections)
	}
	if err := MergeRuntime(cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Connections) != 2 {
		t.Fatalf("merge should be idempotent, got %+v", cfg.Connections)
	}
}

func TestRuntimeExpandsEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BDB_RT_PASSWORD", "secret")
	conn := Connection{Name: "pg", Type: TypePostgres, Host: "h", User: "u", Database: "d", Password: "${BDB_RT_PASSWORD}"}
	if err := AppendRuntimeConnection(conn); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRuntimeConnections()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].Password != "secret" {
		t.Fatalf("password = %q", loaded[0].Password)
	}
	raw, err := loadRuntimeRaw()
	if err != nil {
		t.Fatal(err)
	}
	if raw[0].Password != "${BDB_RT_PASSWORD}" {
		t.Fatalf("raw password should stay unexpanded, got %q", raw[0].Password)
	}
}
