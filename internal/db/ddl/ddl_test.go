package ddl

import (
	"strings"
	"testing"
)

func TestDropTable(t *testing.T) {
	if got := DropTable("postgres", "public", "users"); got != `DROP TABLE "public"."users";` {
		t.Fatalf("unexpected postgres drop: %s", got)
	}
	if got := DropTable("mysql", "db", "users"); got != "DROP TABLE `db`.`users`;" {
		t.Fatalf("unexpected mysql drop: %s", got)
	}
}

func TestTruncateTable(t *testing.T) {
	if got := TruncateTable("postgres", "public", "users"); got != `TRUNCATE TABLE "public"."users";` {
		t.Fatalf("unexpected postgres truncate: %s", got)
	}
	if got := TruncateTable("sqlite", "main", "users"); got != `DELETE FROM "users";` {
		t.Fatalf("unexpected sqlite truncate: %s", got)
	}
}

func TestRenameTable(t *testing.T) {
	if got := RenameTable("mysql", "db", "users", "people"); got != "ALTER TABLE `db`.`users` RENAME TO `people`;" {
		t.Fatalf("unexpected rename: %s", got)
	}
}

func TestAddColumn(t *testing.T) {
	got := AddColumn("postgres", "public", "users", "age", "INTEGER", false, "0")
	want := `ALTER TABLE "public"."users" ADD COLUMN "age" INTEGER NOT NULL DEFAULT 0;`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = AddColumn("sqlite", "main", "users", "note", "TEXT", true, "")
	want = `ALTER TABLE "users" ADD COLUMN "note" TEXT;`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDropColumn(t *testing.T) {
	if got := DropColumn("mysql", "db", "users", "age"); got != "ALTER TABLE `db`.`users` DROP COLUMN `age`;" {
		t.Fatalf("unexpected drop column: %s", got)
	}
}

func TestCreateIndex(t *testing.T) {
	got := CreateIndex("postgres", "public", "users", "users_email", []string{"email", "tenant"}, true)
	want := `CREATE UNIQUE INDEX "users_email" ON "public"."users" ("email", "tenant");`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCreateIndexMySQLQuotesColumns(t *testing.T) {
	got := CreateIndex("mysql", "db", "users", "idx_email", []string{"email"}, false)
	want := "CREATE INDEX `idx_email` ON `db`.`users` (`email`);"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDropIndex(t *testing.T) {
	if got := DropIndex("postgres", "public", "users", "idx"); got != `DROP INDEX "public"."idx";` {
		t.Fatalf("unexpected postgres drop index: %s", got)
	}
	if got := DropIndex("mysql", "db", "users", "idx"); got != "DROP INDEX `idx` ON `db`.`users`;" {
		t.Fatalf("unexpected mysql drop index: %s", got)
	}
	if got := DropIndex("sqlite", "main", "users", "idx"); got != `DROP INDEX "idx";` {
		t.Fatalf("unexpected sqlite drop index: %s", got)
	}
}

func TestCreateTable(t *testing.T) {
	got := CreateTable("postgres", "public", "users", "id INTEGER PRIMARY KEY,\nname TEXT")
	want := "CREATE TABLE \"public\".\"users\" (\n  id INTEGER PRIMARY KEY,\n  name TEXT\n);"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !strings.Contains(got, "name TEXT") {
		t.Fatalf("missing column: %s", got)
	}
}
