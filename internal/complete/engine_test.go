package complete

import "testing"

func TestKeywordsAndTables(t *testing.T) {
	e := New()
	e.SetTables([]string{"users", "orders", "users"})

	got := e.Suggest("sel")
	if len(got) == 0 || got[0].Text != "SELECT" || got[0].Kind != KindKeyword {
		t.Fatalf("suggest(sel) = %+v", got)
	}

	got = e.Suggest("us")
	if len(got) == 0 || got[0].Text != "users" || got[0].Kind != KindTable {
		t.Fatalf("suggest(us) = %+v", got)
	}
}

func TestColumnsAndQualifier(t *testing.T) {
	e := New()
	e.SetTables([]string{"users"})
	if e.HasColumns("users") {
		t.Fatal("columns should not be cached yet")
	}
	e.SetColumns("users", []string{"id", "name", "email"})

	if !e.HasColumns("Users") {
		t.Fatal("column lookup should be case-insensitive")
	}
	got := e.Suggest("users.na")
	if len(got) != 1 || got[0].Text != "name" || got[0].Kind != KindColumn {
		t.Fatalf("suggest(users.na) = %+v", got)
	}
	if got := e.Suggest("unknown."); len(got) != 0 {
		t.Fatalf("unknown qualifier = %+v", got)
	}

	got = e.Suggest("")
	if len(got) == 0 {
		t.Fatal("empty prefix should return candidates")
	}
}

func TestSplitWord(t *testing.T) {
	if q, p := SplitWord("users.na"); q != "users" || p != "na" {
		t.Fatalf("split = %q %q", q, p)
	}
	if q, p := SplitWord("sel"); q != "" || p != "sel" {
		t.Fatalf("split = %q %q", q, p)
	}
}

func TestReset(t *testing.T) {
	e := New()
	e.SetTables([]string{"users"})
	e.SetColumns("users", []string{"id"})
	e.Reset()
	if e.HasColumns("users") {
		t.Fatal("reset should clear cached columns")
	}
	for _, candidate := range e.Suggest("us") {
		if candidate.Kind == KindTable {
			t.Fatalf("reset should clear tables, got %+v", candidate)
		}
	}
}
