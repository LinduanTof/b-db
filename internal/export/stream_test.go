package export

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/LinduanTof/dbrix/internal/db"
	"github.com/LinduanTof/dbrix/internal/db/dialect"
)

type fakeStream struct {
	cols []db.ColumnMeta
	rows [][]any
	next int
}

func (s *fakeStream) Columns() []db.ColumnMeta { return s.cols }

func (s *fakeStream) Next() bool {
	if s.next >= len(s.rows) {
		return false
	}
	s.next++
	return true
}

func (s *fakeStream) Values() ([]any, error) { return s.rows[s.next-1], nil }

func (s *fakeStream) Err() error { return nil }

func (s *fakeStream) Close() error { return nil }

func fakeRows() *fakeStream {
	return &fakeStream{
		cols: []db.ColumnMeta{{Name: "id"}, {Name: "name"}},
		rows: [][]any{{int64(1), "alice"}, {int64(2), "bob"}},
	}
}

func TestCSVStream(t *testing.T) {
	var b strings.Builder
	if err := CSVStream(&b, fakeRows()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "id,name") || !strings.Contains(out, "1,alice") {
		t.Fatalf("csv = %q", out)
	}
}

func TestJSONStream(t *testing.T) {
	var b strings.Builder
	if err := JSONStream(&b, fakeRows()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{`"id": 1`, `"name": "alice"`, `"name": "bob"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json missing %q: %s", want, out)
		}
	}
}

func TestSQLTableStream(t *testing.T) {
	var b strings.Builder
	ddl := `CREATE TABLE "users" ("id" int)`
	if err := SQLTableStream(&b, dialect.Postgres{}, "public", "users", ddl, fakeRows()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`-- "public"."users"`,
		`CREATE TABLE "users" ("id" int);`,
		`INSERT INTO "public"."users" ("id", "name") VALUES (1, 'alice');`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("sql missing %q:\n%s", want, out)
		}
	}
}

func TestJSONTablesStream(t *testing.T) {
	var b strings.Builder
	if err := JSONTablesStream(&b, []TableStream{
		{Schema: "main", Table: "users", Open: func() (db.RowStream, error) { return fakeRows(), nil }},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"users": [{"id": 1, "name": "alice"}`) {
		t.Fatalf("json = %s", b.String())
	}
}

func TestXLSXStream(t *testing.T) {
	var buf bytes.Buffer
	if err := XLSXStream(&buf, []SheetSource{
		{Name: "users", Open: func() (db.RowStream, error) { return fakeRows(), nil }},
	}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var sheet string
	for _, file := range reader.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			sheet = readZipFile(t, file)
		}
	}
	if sheet == "" {
		t.Fatal("missing sheet1")
	}
	if !strings.Contains(sheet, "alice") || !strings.Contains(sheet, "<v>1</v>") {
		t.Fatalf("sheet = %s", sheet)
	}
}
