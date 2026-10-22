package dialect

import "testing"

func TestQuote(t *testing.T) {
	if got := (Postgres{}).Quote(`we"ird`); got != `"we""ird"` {
		t.Fatalf("postgres quote = %s", got)
	}
	if got := (SQLite{}).Quote("name"); got != `"name"` {
		t.Fatalf("sqlite quote = %s", got)
	}
}

func TestPlaceholder(t *testing.T) {
	if got := (Postgres{}).Placeholder(3); got != "$3" {
		t.Fatalf("postgres placeholder = %s", got)
	}
	if got := (SQLite{}).Placeholder(3); got != "?" {
		t.Fatalf("sqlite placeholder = %s", got)
	}
}

func TestQualified(t *testing.T) {
	if got := (Postgres{}).Qualified("public", "users"); got != `"public"."users"` {
		t.Fatalf("postgres qualified = %s", got)
	}
	if got := (SQLite{}).Qualified("main", "users"); got != `"users"` {
		t.Fatalf("sqlite qualified = %s", got)
	}
	if got := (MySQL{}).Qualified("app", "users"); got != "`app`.`users`" {
		t.Fatalf("mysql qualified = %s", got)
	}
}

func TestMySQLQuote(t *testing.T) {
	if got := (MySQL{}).Quote("we`ird"); got != "`we``ird`" {
		t.Fatalf("mysql quote = %s", got)
	}
	if got := (MySQL{}).Placeholder(1); got != "?" {
		t.Fatalf("mysql placeholder = %s", got)
	}
}
