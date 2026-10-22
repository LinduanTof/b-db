package query

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/linduan-guan/b-db/internal/db"
)

type Executor struct {
	Conn        db.Conn
	MaxRows     int
	AlwaysQuery bool
}

type Outcome struct {
	Result  *db.ResultSet
	Exec    *db.ExecResult
	Elapsed time.Duration
	Err     error
}

func (e Executor) Run(ctx context.Context, stmt string) Outcome {
	start := time.Now()
	if e.AlwaysQuery || IsQuery(stmt) {
		rs, err := e.Conn.Querier().Query(ctx, stmt, e.MaxRows)
		return Outcome{Result: rs, Err: err, Elapsed: time.Since(start)}
	}
	res, err := e.Conn.Querier().Exec(ctx, stmt)
	return Outcome{Exec: res, Err: err, Elapsed: time.Since(start)}
}

var queryKeywords = map[string]bool{
	"SELECT":   true,
	"WITH":     true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
	"VALUES":   true,
	"PRAGMA":   true,
	"TABLE":    true,
}

func IsQuery(stmt string) bool {
	return queryKeywords[leadingKeyword(stmt)]
}

func leadingKeyword(stmt string) string {
	s := stmt
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return ""
		}
		break
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '('
	})
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}
