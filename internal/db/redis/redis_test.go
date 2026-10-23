package redis

import "testing"

func TestSplitCommand(t *testing.T) {
	got, err := splitCommand(`SET "my key" 'hello world'`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "SET" || got[1] != "my key" || got[2] != "hello world" {
		t.Fatalf("args = %#v", got)
	}
	if _, err := splitCommand(`GET "unterminated`); err == nil {
		t.Fatal("expected quote error")
	}
}

func TestConvert(t *testing.T) {
	rs := convert("GET", "hello", 10)
	if len(rs.Rows) != 1 || rs.Rows[0][0] != "hello" {
		t.Fatalf("GET = %#v", rs)
	}

	rs = convert("HGETALL", []any{"a", "1", "b", "2"}, 10)
	if len(rs.Rows) != 2 || rs.Rows[0][0] != "a" || rs.Rows[1][1] != "2" {
		t.Fatalf("HGETALL = %#v", rs)
	}

	rs = convert("LRANGE", []any{"a", "b", "c"}, 2)
	if len(rs.Rows) != 2 {
		t.Fatalf("LRANGE rows = %#v", rs.Rows)
	}
}

func TestSplitCommandKeepsEmptyArgument(t *testing.T) {
	got, err := splitCommand(`SET key ""`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "SET" || got[1] != "key" || got[2] != "" {
		t.Fatalf("args = %#v", got)
	}
}

func TestConvertTruncated(t *testing.T) {
	rs := convert("LRANGE", []any{"a", "b", "c"}, 2)
	if !rs.Truncated {
		t.Fatal("list result should be marked truncated")
	}
	rs = convert("HGETALL", []any{"a", "1", "b", "2", "c", "3"}, 2)
	if !rs.Truncated || len(rs.Rows) != 2 {
		t.Fatalf("hash result = %#v", rs)
	}
}

func FuzzSplitCommand(f *testing.F) {
	f.Add(`SET "my key" 'hello world'`)
	f.Add(`GET ""`)
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = splitCommand(input)
	})
}
