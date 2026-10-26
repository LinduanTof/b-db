package complete

import "testing"

func TestAnalyzeContexts(t *testing.T) {
	cases := []struct {
		sql     string
		kind    ContextKind
		trigger bool
	}{
		{"SELECT * FROM ", ContextTable, true},
		{"SELECT * FROM us", ContextTable, true},
		{"SELECT * FROM users, ", ContextTable, true},
		{"INSERT INTO ", ContextTable, true},
		{"UPDATE ", ContextTable, true},
		{"SELECT * ", ContextColumn, false},
		{"SELECT * FROM users WHERE ", ContextColumn, true},
		{"SELECT * FROM users WHERE id = ", ContextColumn, true},
		{"SELECT * FROM users WHERE id AND ", ContextColumn, true},
		{"SELECT * FROM users ORDER BY ", ContextColumn, true},
		{"SELECT * FROM users u JOIN orders o ON ", ContextColumn, true},
	}
	for _, tc := range cases {
		ctx := Analyze(tc.sql, len([]rune(tc.sql)))
		if ctx.Kind != tc.kind || ctx.Trigger != tc.trigger {
			t.Fatalf("Analyze(%q) = kind %v trigger %v, want %v %v",
				tc.sql, ctx.Kind, ctx.Trigger, tc.kind, tc.trigger)
		}
	}
}

func TestAnalyzeTables(t *testing.T) {
	ctx := Analyze("SELECT u.name FROM public.users AS u JOIN orders o ON u.id = o.user_id WHERE ", 0)
	// Analyze uses cursor 0; tables still come from the whole statement.
	if len(ctx.Tables) != 2 {
		t.Fatalf("tables = %+v", ctx.Tables)
	}
	users := ctx.Tables[0]
	if users.Schema != "public" || users.Table != "users" || users.Alias != "u" {
		t.Fatalf("users ref = %+v", users)
	}
	orders := ctx.Tables[1]
	if orders.Table != "orders" || orders.Alias != "o" {
		t.Fatalf("orders ref = %+v", orders)
	}
}

func TestAnalyzeStatementBoundaries(t *testing.T) {
	sql := "SELECT * FROM old; SELECT * FROM "
	ctx := Analyze(sql, len([]rune(sql)))
	if ctx.Kind != ContextTable || !ctx.Trigger {
		t.Fatalf("context = %+v", ctx)
	}
	if len(ctx.Tables) != 0 {
		t.Fatalf("tables = %+v, want none", ctx.Tables)
	}

	sql = "SELECT ';' FROM users WHERE "
	ctx = Analyze(sql, len([]rune(sql)))
	if len(ctx.Tables) != 1 || ctx.Tables[0].Table != "users" {
		t.Fatalf("tables = %+v", ctx.Tables)
	}
}

func TestResolveTable(t *testing.T) {
	ctx := Context{Tables: []TableRef{{Schema: "public", Table: "users", Alias: "u"}}}
	for _, qualifier := range []string{"u", "users", "public.users", "U"} {
		if table, ok := ResolveTable(ctx, qualifier); !ok || table != "users" {
			t.Fatalf("ResolveTable(%q) = %q %v", qualifier, table, ok)
		}
	}
	if _, ok := ResolveTable(ctx, "orders"); ok {
		t.Fatal("unknown qualifier should not resolve")
	}
}

func TestAnalyzeDoesNotTriggerAfterValues(t *testing.T) {
	cases := []string{
		"SELECT * FROM users WHERE name = 'alice' ",
		"SELECT * FROM users WHERE id = 1 ",
		"SELECT * FROM users WHERE id IN (1, 2) ",
	}
	for _, sql := range cases {
		ctx := Analyze(sql, len([]rune(sql)))
		if ctx.Trigger {
			t.Fatalf("Analyze(%q) should not trigger", sql)
		}
		if ctx.Kind != ContextColumn {
			t.Fatalf("Analyze(%q) kind = %v", sql, ctx.Kind)
		}
	}
}
