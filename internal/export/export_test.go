package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LinduanTof/dbrix/internal/db"
)

func sample() *db.ResultSet {
	return &db.ResultSet{
		Columns: []db.ColumnMeta{
			{Name: "id"},
			{Name: "name"},
			{Name: "note"},
			{Name: "missing"},
		},
		Rows: [][]any{
			{int64(1), "张三", "a,b", nil},
			{int64(2), `say "hi"`, "line1\nline2", "x"},
		},
	}
}

func TestCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	want := "id,name,note,missing\n1,张三,\"a,b\",\n2,\"say \"\"hi\"\"\",\"line1\nline2\",x\n"
	if buf.String() != want {
		t.Fatalf("CSV mismatch:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0]["missing"] != nil {
		t.Fatalf("missing should be null, got %#v", got[0]["missing"])
	}
	if got[0]["name"] != "张三" {
		t.Fatalf("name = %#v", got[0]["name"])
	}
	if !strings.Contains(buf.String(), `"id": 1`) {
		t.Fatalf("expected column order in output:\n%s", buf.String())
	}
}

func TestMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := Markdown(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"| id | name | note | missing |",
		"| --- | --- | --- | --- |",
		`| 1 | 张三 | a,b |  |`,
		`say "hi"`,
		"line1<br>line2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
}

func TestMarkdownEscapesPipes(t *testing.T) {
	rs := &db.ResultSet{
		Columns: []db.ColumnMeta{{Name: "a|b"}},
		Rows:    [][]any{{"x|y"}},
	}
	var buf bytes.Buffer
	if err := Markdown(&buf, rs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `a\|b`) || !strings.Contains(buf.String(), `x\|y`) {
		t.Fatalf("pipes not escaped:\n%s", buf.String())
	}
}

func TestEmptyJSON(t *testing.T) {
	var buf bytes.Buffer
	err := JSON(&buf, &db.ResultSet{Columns: []db.ColumnMeta{{Name: "id"}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("empty JSON = %q", buf.String())
	}
}
