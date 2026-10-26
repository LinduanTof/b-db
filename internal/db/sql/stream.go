package sql

import (
	"context"
	stdsql "database/sql"

	"github.com/LinduanTof/dbrix/internal/db"
)

func (c *conn) Stream(ctx context.Context, stmt string, maxRows int) (db.RowStream, error) {
	rows, err := c.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	names, err := rows.Columns()
	if err != nil {
		rows.Close()
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
	return &rowStream{rows: rows, cols: cols, maxRows: maxRows}, nil
}

type rowStream struct {
	rows    *stdsql.Rows
	cols    []db.ColumnMeta
	maxRows int
	count   int
}

func (s *rowStream) Columns() []db.ColumnMeta { return s.cols }

func (s *rowStream) Next() bool {
	if s.maxRows > 0 && s.count >= s.maxRows {
		return false
	}
	return s.rows.Next()
}

func (s *rowStream) Values() ([]any, error) {
	values := make([]any, len(s.cols))
	ptrs := make([]any, len(values))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := s.rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	for i, value := range values {
		values[i] = normalize(value)
	}
	s.count++
	return values, nil
}

func (s *rowStream) Err() error { return s.rows.Err() }

func (s *rowStream) Close() error { return s.rows.Close() }
