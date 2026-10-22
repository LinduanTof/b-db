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
