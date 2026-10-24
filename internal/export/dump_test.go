package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/dialect"
)

func TestSQLTable(t *testing.T) {
	rs := &db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "id"}, {Name: "name"}, {Name: "active"}},
		Rows: [][]any{
			{int64(1), "O'Reilly", true},
			{int64(2), nil, false},
		},
	}
	var b strings.Builder
	if err := SQLTable(&b, dialect.Postgres{}, "public", "users", `CREATE TABLE "users" ("id" int)`, rs); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`-- "public"."users"`,
		`CREATE TABLE "users" ("id" int);`,
		`INSERT INTO "public"."users" ("id", "name", "active") VALUES (1, 'O''Reilly', TRUE);`,
		`INSERT INTO "public"."users" ("id", "name", "active") VALUES (2, NULL, FALSE);`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dump missing %q:\n%s", want, out)
		}
	}
}

func TestXLSX(t *testing.T) {
	var buf bytes.Buffer
	sheets := []Sheet{
		{Name: "users", Columns: []string{"id", "name"}, Rows: [][]any{{int64(1), "alice"}, {int64(2), nil}}},
		{Name: "orders", Columns: []string{"id"}, Rows: [][]any{{int64(3)}}},
	}
	if err := XLSX(&buf, sheets); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	for _, want := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if files[want] == nil {
			t.Fatalf("missing %q in xlsx", want)
		}
	}
	sheet := readZipFile(t, files["xl/worksheets/sheet1.xml"])
	if !strings.Contains(sheet, "alice") || !strings.Contains(sheet, "<v>1</v>") {
		t.Fatalf("sheet content = %s", sheet)
	}
	workbook := readZipFile(t, files["xl/workbook.xml"])
	if !strings.Contains(workbook, `name="users"`) || !strings.Contains(workbook, `name="orders"`) {
		t.Fatalf("workbook = %s", workbook)
	}
}

func readZipFile(t *testing.T, file *zip.File) string {
	t.Helper()
	rc, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestJSONTables(t *testing.T) {
	rs := &db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "id"}, {Name: "name"}},
		Rows:    [][]any{{int64(1), "alice"}},
	}
	var b strings.Builder
	if err := JSONTables(&b, []TableData{
		{Schema: "main", Table: "users", Result: rs},
		{Schema: "public", Table: "orders", Result: nil},
	}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{`"users": [{"id": 1, "name": "alice"}]`, `"public.orders": []`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json missing %q:\n%s", want, out)
		}
	}
}
