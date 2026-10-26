package importer

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readAll(t *testing.T, r Reader) [][]any {
	t.Helper()
	var rows [][]any
	for {
		row, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestCSVWithHeader(t *testing.T) {
	path := writeFile(t, "data.csv", "id,name\n1,alice\n2,bob\n")
	r, err := Open(path, FormatCSV, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := r.Columns(); len(got) != 2 || got[0] != "id" || got[1] != "name" {
		t.Fatalf("unexpected columns: %v", got)
	}
	rows := readAll(t, r)
	if len(rows) != 2 || rows[1][1] != "bob" {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestCSVWithoutHeader(t *testing.T) {
	path := writeFile(t, "data.csv", "1,alice\n2,bob\n")
	r, err := Open(path, FormatCSV, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.Columns()) != 0 {
		t.Fatalf("expected no columns, got %v", r.Columns())
	}
	if rows := readAll(t, r); len(rows) != 2 {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestCSVDelimiterDetection(t *testing.T) {
	for name, content := range map[string]string{
		"semicolon": "id;name\n1;alice\n",
		"tab":       "id\tname\n1\talice\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, "data.csv", content)
			r, err := Open(path, FormatCSV, true)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			rows := readAll(t, r)
			if len(rows) != 1 || len(rows[0]) != 2 {
				t.Fatalf("unexpected rows: %v", rows)
			}
		})
	}
}

func TestJSONArray(t *testing.T) {
	path := writeFile(t, "data.json", `[{"id":1,"name":"alice"},{"id":2,"nick":"bob"}]`)
	r, err := Open(path, FormatJSON, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	columns := r.Columns()
	if len(columns) != 3 || columns[0] != "id" || columns[1] != "name" || columns[2] != "nick" {
		t.Fatalf("unexpected columns: %v", columns)
	}
	rows := readAll(t, r)
	if len(rows) != 2 {
		t.Fatalf("unexpected rows: %v", rows)
	}
	if rows[1][1] != nil || rows[1][2] != "bob" {
		t.Fatalf("missing keys should be nil: %v", rows[1])
	}
}

func TestJSONRejectsObject(t *testing.T) {
	path := writeFile(t, "data.json", `{"id":1}`)
	if _, err := Open(path, FormatJSON, false); err == nil {
		t.Fatal("expected an error for a JSON object")
	}
}
