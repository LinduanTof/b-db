package sql

import (
	"context"
	stdsql "database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/dialect"
)

func TestMutatorSQLite(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (id, name) VALUES (1, 'a'), (2, 'b')`); err != nil {
		t.Fatal(err)
	}

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	mutator := c.Mutator()
	ctx := context.Background()
	table := db.TableRef{Schema: "main", Name: "users"}
	key := map[string]any{"id": int64(1)}

	affected, err := mutator.UpdateCell(ctx, table, key, "name", "alice")
	if err != nil {
		t.Fatalf("UpdateCell: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	var name string
	if err := sqlDB.QueryRow(`SELECT name FROM users WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "alice" {
		t.Fatalf("name = %q, want alice", name)
	}

	preview := mutator.PreviewUpdate(table, key, "name", "bob")
	if !strings.Contains(preview, "UPDATE") || !strings.Contains(preview, "WHERE") || !strings.Contains(preview, "bob") {
		t.Fatalf("preview = %q", preview)
	}

	affected, err = mutator.DeleteRow(ctx, table, key)
	if err != nil {
		t.Fatalf("DeleteRow: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, want 1", affected)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestMutatorRejectsMissingKey(t *testing.T) {
	mutator := mutator{c: &conn{}}
	if _, err := mutator.UpdateCell(context.Background(), db.TableRef{Name: "t"}, nil, "x", 1); err == nil {
		t.Fatal("expected error for missing primary key")
	}
	if _, err := mutator.DeleteRow(context.Background(), db.TableRef{Name: "t"}, nil); err == nil {
		t.Fatal("expected error for missing primary key")
	}
}

func TestMutatorRejectsNoMatch(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	_, err = c.Mutator().UpdateCell(context.Background(), db.TableRef{Name: "t"}, map[string]any{"id": int64(99)}, "v", "x")
	if err == nil {
		t.Fatal("expected affected-rows error")
	}
}

func TestInsertRows(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	inserter, ok := c.Mutator().(db.Inserter)
	if !ok {
		t.Fatal("sqlite mutator should implement db.Inserter")
	}
	ctx := context.Background()
	table := db.TableRef{Schema: "main", Name: "users"}

	var progress []int64
	rows := db.NewSliceRows([][]any{{int64(1), "a"}, {int64(2), "b"}})
	affected, err := inserter.InsertRows(ctx, table, []string{"id", "name"}, rows, func(n int64) {
		progress = append(progress, n)
	})
	if err != nil {
		t.Fatalf("InsertRows: %v", err)
	}
	if affected != 2 {
		t.Fatalf("affected = %d, want 2", affected)
	}
	if len(progress) != 2 || progress[0] != 1 || progress[1] != 2 {
		t.Fatalf("progress = %v", progress)
	}

	// The second row violates NOT NULL, so the whole batch must roll back.
	bad := db.NewSliceRows([][]any{{int64(3), "c"}, {int64(4), nil}})
	if _, err := inserter.InsertRows(ctx, table, []string{"id", "name"}, bad, nil); err == nil {
		t.Fatal("expected a NOT NULL error")
	}
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 after rollback", count)
	}

	preview := inserter.PreviewInsert(table, []string{"name"}, [][]any{{"alice"}, {nil}})
	if !strings.Contains(preview, "INSERT INTO") || !strings.Contains(preview, "'alice'") || !strings.Contains(preview, "NULL") {
		t.Fatalf("preview = %q", preview)
	}
	if strings.Contains(preview, "?") {
		t.Fatalf("preview must not contain placeholders: %q", preview)
	}
}
