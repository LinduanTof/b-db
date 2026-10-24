package sql

import (
	"context"
	stdsql "database/sql"
	"path/filepath"
	"testing"

	"github.com/LinduanTof/b-db/internal/db/dialect"
)

func TestSQLiteStream(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (id, name) VALUES (1, 'a'), (2, 'b'), (3, 'c')`); err != nil {
		t.Fatal(err)
	}

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	stream, err := c.Stream(context.Background(), "SELECT id, name FROM users ORDER BY id", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if columns := stream.Columns(); len(columns) != 2 || columns[0].Name != "id" || columns[1].Name != "name" {
		t.Fatalf("columns = %#v", columns)
	}
	var names []string
	for stream.Next() {
		values, err := stream.Values()
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, values[1].(string))
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[2] != "c" {
		t.Fatalf("names = %#v", names)
	}
}

func TestSQLiteStreamLimit(t *testing.T) {
	sqlDB, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO t (id) VALUES (1), (2), (3)`); err != nil {
		t.Fatal(err)
	}

	c := newConn("test", "sqlite", sqlDB, dialect.SQLite{})
	stream, err := c.Stream(context.Background(), "SELECT id FROM t", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	count := 0
	for stream.Next() {
		if _, err := stream.Values(); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("rows = %d, want 2", count)
	}
}
