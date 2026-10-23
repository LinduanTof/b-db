package sqlscript

import (
	"strings"
	"testing"
)

func TestSplitBasic(t *testing.T) {
	got, err := Split("select 1; select 2;\nselect 3")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "select 1" || got[2] != "select 3" {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitIgnoresSemicolonsInStringsAndComments(t *testing.T) {
	script := `
-- create; not a statement
select 'a;b' as x;
/* block; comment */
insert into t values ("c;d");
select ` + "`we;ird`" + ` from t;
`
	got, err := Split(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("statements (%d) = %#v", len(got), got)
	}
	if !strings.Contains(got[0], "'a;b'") {
		t.Fatalf("statement 0 = %q", got[0])
	}
	if !strings.Contains(got[1], `"c;d"`) {
		t.Fatalf("statement 1 = %q", got[1])
	}
	if !strings.Contains(got[2], "`we;ird`") {
		t.Fatalf("statement 2 = %q", got[2])
	}
}

func TestSplitEscapedQuotes(t *testing.T) {
	got, err := Split("select 'it''s'; select 'a\\'b';")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitErrors(t *testing.T) {
	if _, err := Split("select 'unterminated"); err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if _, err := Split("select 1 /* open"); err == nil {
		t.Fatal("expected unterminated comment error")
	}
}

func TestSplitEmpty(t *testing.T) {
	got, err := Split("  -- only comments\n ;; ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitForPostgresKeepsHashOperator(t *testing.T) {
	got, err := SplitFor("SELECT 1 # 2; SELECT 3;", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "#") {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitForPostgresDollarQuote(t *testing.T) {
	script := "CREATE FUNCTION f() RETURNS void AS $$ BEGIN RAISE NOTICE 'a;b'; END; $$ LANGUAGE plpgsql;\nSELECT 1;"
	got, err := SplitFor(script, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "a;b") || got[1] != "SELECT 1" {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitForMySQLExtensions(t *testing.T) {
	script := "# comment; ignored\nDELIMITER //\nCREATE PROCEDURE p() BEGIN SELECT 1; END//\nDELIMITER ;\nSELECT 2;"
	got, err := SplitFor(script, "mysql")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "SELECT 1;") || got[1] != "SELECT 2" {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitForSQLiteHashIsNotComment(t *testing.T) {
	got, err := SplitFor("SELECT 1 # 2;", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "#") {
		t.Fatalf("statements = %#v", got)
	}
}

func TestSplitForUnterminatedDollarQuote(t *testing.T) {
	if _, err := SplitFor("SELECT $$ open", "postgres"); err == nil {
		t.Fatal("expected unterminated dollar quote error")
	}
}

func FuzzSplitFor(f *testing.F) {
	f.Add("SELECT 1; SELECT 2;", "postgres")
	f.Add("DELIMITER //\nSELECT 1//", "mysql")
	f.Add("SELECT 'a;b';", "sqlite")
	f.Fuzz(func(t *testing.T, script, dialect string) {
		_, _ = SplitFor(script, dialect)
	})
}
