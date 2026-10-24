package sql

import (
	"context"
	stdsql "database/sql"
	"path/filepath"
	"testing"

	"github.com/LinduanTof/b-db/internal/db/dialect"
)

func TestSQLiteVersion(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	version, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version == "" {
		t.Fatal("empty sqlite version")
	}
}

func TestSQLiteTableStatsUnsupported(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	stats, err := c.TableStats(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if stats != nil {
		t.Fatalf("sqlite stats = %#v, want nil", stats)
	}
}
