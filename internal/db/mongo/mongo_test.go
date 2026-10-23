package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParseCommand(t *testing.T) {
	req, err := parseCommand(`find mydb.users {"name":"Alice"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.op != "find" || req.database != "mydb" || req.collection != "users" {
		t.Fatalf("request = %+v", req)
	}
	if req.filter["name"] != "Alice" {
		t.Fatalf("filter = %#v", req.filter)
	}

	req, err = parseCommand(`aggregate mydb.users [{"$match":{"name":"Alice"}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if req.op != "aggregate" || len(req.pipeline) != 1 {
		t.Fatalf("request = %+v", req)
	}

	req, err = parseCommand("count mydb.users")
	if err != nil {
		t.Fatal(err)
	}
	if req.op != "count" || len(req.filter) != 0 {
		t.Fatalf("request = %+v", req)
	}
}

func TestParseCommandErrors(t *testing.T) {
	for _, stmt := range []string{"", "find", "find users {}", "find mydb.users {bad"} {
		if _, err := parseCommand(stmt); err == nil {
			t.Fatalf("expected error for %q", stmt)
		}
	}
}

func TestDocumentsToResult(t *testing.T) {
	docs := []bson.M{
		{"name": "Alice", "age": int32(30)},
		{"name": "Bob", "active": true},
	}
	rs := documentsToResult(docs, 10)
	if len(rs.Columns) != 3 {
		t.Fatalf("columns = %+v", rs.Columns)
	}
	index := map[string]int{}
	for i, column := range rs.Columns {
		index[column.Name] = i
	}
	if len(rs.Rows) != 2 || rs.Rows[0][index["name"]] != "Alice" {
		t.Fatalf("rows = %#v", rs.Rows)
	}
	if rs.Rows[1][index["active"]] != true {
		t.Fatalf("active = %#v", rs.Rows[1][index["active"]])
	}
}

func TestFormatMongoValueNormalizesInt32(t *testing.T) {
	if got := formatMongoValue(int32(30)); got != int64(30) {
		t.Fatalf("int32 = %#v", got)
	}
}

func FuzzParseCommand(f *testing.F) {
	f.Add(`find mydb.users {"name":"Alice"}`)
	f.Add(`aggregate mydb.users [{"$match":{"name":"Alice"}}]`)
	f.Add(`count mydb.users`)
	f.Fuzz(func(t *testing.T, stmt string) {
		_, _ = parseCommand(stmt)
	})
}
