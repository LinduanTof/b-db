package query

import "testing"

func TestIsQuery(t *testing.T) {
	cases := []struct {
		stmt string
		want bool
	}{
		{"SELECT 1", true},
		{"  select * from users", true},
		{"-- comment\nSELECT 1", true},
		{"/* note */ WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"PRAGMA table_info(users)", true},
		{"EXPLAIN SELECT 1", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a = 1", false},
		{"CREATE TABLE t (id int)", false},
		{"", false},
		{"-- only a comment", false},
	}
	for _, tc := range cases {
		if got := IsQuery(tc.stmt); got != tc.want {
			t.Errorf("IsQuery(%q) = %v, want %v", tc.stmt, got, tc.want)
		}
	}
}

func TestIsQueryWithReturningAndCTEs(t *testing.T) {
	cases := []struct {
		stmt string
		want bool
	}{
		{"INSERT INTO t VALUES (1) RETURNING id", true},
		{"UPDATE t SET a = 1 RETURNING a", true},
		{"WITH x AS (SELECT 1) DELETE FROM t", false},
		{"WITH x AS (SELECT 1) INSERT INTO t SELECT * FROM x", false},
		{"WITH x AS (SELECT 1) INSERT INTO t SELECT * FROM x RETURNING id", true},
		{"(SELECT 1)", true},
		{"SELECT $$ DELETE $$", true},
	}
	for _, tc := range cases {
		if got := IsQuery(tc.stmt); got != tc.want {
			t.Errorf("IsQuery(%q) = %v, want %v", tc.stmt, got, tc.want)
		}
	}
}

func TestIsWrite(t *testing.T) {
	cases := []struct {
		stmt string
		want bool
	}{
		{"SELECT 1", false},
		{"SELECT 'DELETE'", false},
		{"-- DELETE\nSELECT 1", false},
		{"SELECT $$ UPDATE $$", false},
		{"WITH x AS (SELECT 1) DELETE FROM t", true},
		{"INSERT INTO t VALUES (1)", true},
		{"SELECT * FROM t FOR UPDATE", true},
		{"SET key value", true},
		{"GET key", false},
	}
	for _, tc := range cases {
		if got := IsWrite(tc.stmt); got != tc.want {
			t.Errorf("IsWrite(%q) = %v, want %v", tc.stmt, got, tc.want)
		}
	}
}

func TestIsWriteFor(t *testing.T) {
	cases := []struct {
		dialect string
		stmt    string
		want    bool
	}{
		{"sqlite", "SELECT 1", false},
		{"sqlite", "PRAGMA table_info(users)", false},
		{"sqlite", "PRAGMA main.table_info(users)", false},
		{"sqlite", "PRAGMA journal_mode = WAL", true},
		{"sqlite", "PRAGMA user_version = 1", true},
		{"sqlite", "PRAGMA table_info(users); PRAGMA journal_mode = WAL", true},
		{"sqlite", "SELECT 1; PRAGMA journal_mode = WAL", true},
		{"sqlite", "EXECUTE 'DELETE FROM users'", true},
		{"postgres", "UPDATE users SET name = 'x'", true},
		{"postgres", "EXECUTE stmt", true},
		{"mysql", "/*! DELETE FROM users */", true},
		{"mysql", "/*!50000 DELETE FROM users */", true},
		{"mysql", "SELECT 1 /*! ; DELETE FROM users */", true},
		{"mysql", "SELECT 1 /* normal */ /*! DELETE FROM users */", true},
		{"mysql", "/*! SELECT 1 */", false},

		{"redis", "GET key", false},
		{"redis", "HGETALL key", false},
		{"redis", "SCAN 0 COUNT 10", false},
		{"redis", "SORT key", false},
		{"redis", "SORT_RO key", false},
		{"redis", "SORT key STORE dest", true},
		{"redis", "SET key value", true},
		{"redis", "SETBIT key 0 1", true},
		{"redis", "GETSET key value", true},
		{"redis", "EVAL \"return 1\" 0", true},
		{"redis", "EVAL_RO \"return 1\" 0", false},
		{"redis", "FLUSHALL", true},
		{"redis", "CONFIG GET maxmemory", true},
		{"redis", "UNKNOWNCOMMAND", true},

		{"mongo", "find db.users {}", false},
		{"mongo", `aggregate db.users [{"$match":{}}]`, false},
		{"mongo", `aggregate db.users [{"$out":"archive"}]`, true},
		{"mongo", "aggregate db.users [{\"\\u0024out\":\"archive\"}]", true},
		{"mongo", `find db.users {"note":"$out"}`, false},
		{"mongo", `aggregate db.users [{"$merge":{"into":"archive"}}]`, true},
		{"mongo", `find db.users {"$where":"this.x > 1"}`, true},
		{"mongo", `count db.users {"$function":{}}`, true},
	}
	for _, tc := range cases {
		if got := IsWriteFor(tc.dialect, tc.stmt); got != tc.want {
			t.Errorf("IsWriteFor(%q, %q) = %v, want %v", tc.dialect, tc.stmt, got, tc.want)
		}
	}
}

func FuzzIsQueryAndWrite(f *testing.F) {
	for _, seed := range []string{"SELECT 1", "INSERT INTO t VALUES (1) RETURNING id", "WITH x AS (SELECT 'DELETE') SELECT 1", "$tag$ x $tag$"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, stmt string) {
		_ = IsQuery(stmt)
		_ = IsWrite(stmt)
		_ = IsWriteFor("sqlite", stmt)
		_ = IsWriteFor("redis", stmt)
		_ = IsWriteFor("mongo", stmt)
	})
}
