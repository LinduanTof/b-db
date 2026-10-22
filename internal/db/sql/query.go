package sql

import (
	"context"
	"time"

	"github.com/linduan/bdb/internal/db"
)

type querier struct{ c *conn }

func (q querier) Query(ctx context.Context, stmt string, maxRows int) (*db.ResultSet, error) {
	start := time.Now()
	rows, err := q.c.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, _ := rows.ColumnTypes()
	cols := make([]db.ColumnMeta, len(names))
	for i, name := range names {
		meta := db.ColumnMeta{Name: name}
		if i < len(types) && types[i] != nil {
			meta.DatabaseType = types[i].DatabaseTypeName()
			if nullable, ok := types[i].Nullable(); ok {
				meta.Nullable = nullable
			}
		}
		cols[i] = meta
	}

	out := make([][]any, 0, 64)
	truncated := false
	for rows.Next() {
		if maxRows > 0 && len(out) == maxRows {
			truncated = true
			break
		}
		vals := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			vals[i] = normalize(v)
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &db.ResultSet{
		Columns:   cols,
		Rows:      out,
		Truncated: truncated,
		Elapsed:   time.Since(start),
	}, nil
}

func (q querier) Exec(ctx context.Context, stmt string) (*db.ExecResult, error) {
	start := time.Now()
	res, err := q.c.db.ExecContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		affected = 0
	}
	return &db.ExecResult{Affected: affected, Elapsed: time.Since(start)}, nil
}

func normalize(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339Nano)
	default:
		return v
	}
}
