package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"

	"github.com/LinduanTof/b-db/internal/db"
)

func (c *conn) Version(ctx context.Context) (string, error) {
	var stmt string
	switch c.typ {
	case "postgres":
		stmt = "SHOW server_version"
	case "mysql":
		stmt = "SELECT VERSION()"
	case "sqlite":
		stmt = "SELECT sqlite_version()"
	default:
		return "", fmt.Errorf("version not implemented for %s", c.typ)
	}
	var version string
	if err := c.db.QueryRowContext(ctx, stmt).Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func (c *conn) TableStats(ctx context.Context, schema string) ([]db.TableStat, error) {
	switch c.typ {
	case "postgres":
		return c.pgTableStats(ctx, schema)
	case "mysql":
		return c.mysqlTableStats(ctx, schema)
	default:
		return nil, nil
	}
}

func (c *conn) pgTableStats(ctx context.Context, schema string) ([]db.TableStat, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT c.relname,
		       CASE WHEN c.reltuples < 0 THEN NULL ELSE c.reltuples::bigint END,
		       pg_total_relation_size(c.oid),
		       (SELECT count(*) FROM information_schema.columns col
		         WHERE col.table_schema = n.nspname AND col.table_name = c.relname),
		       COALESCE(obj_description(c.oid, 'pg_class'), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r','p','v','m')
		ORDER BY c.relname`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTableStats(rows)
}

func (c *conn) mysqlTableStats(ctx context.Context, schema string) ([]db.TableStat, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT t.TABLE_NAME,
		       t.TABLE_ROWS,
		       COALESCE(t.DATA_LENGTH, 0) + COALESCE(t.INDEX_LENGTH, 0),
		       (SELECT COUNT(*) FROM information_schema.COLUMNS col
		         WHERE col.TABLE_SCHEMA = t.TABLE_SCHEMA AND col.TABLE_NAME = t.TABLE_NAME),
		       CASE WHEN t.TABLE_TYPE = 'VIEW' AND t.TABLE_COMMENT = 'VIEW'
		            THEN '' ELSE COALESCE(t.TABLE_COMMENT, '') END
		FROM information_schema.TABLES t
		WHERE t.TABLE_SCHEMA = ?
		ORDER BY t.TABLE_NAME`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTableStats(rows)
}

func scanTableStats(rows *stdsql.Rows) ([]db.TableStat, error) {
	var out []db.TableStat
	for rows.Next() {
		var (
			name, comment           string
			rowCount, size, columns stdsql.NullInt64
		)
		if err := rows.Scan(&name, &rowCount, &size, &columns, &comment); err != nil {
			return nil, err
		}
		out = append(out, db.TableStat{
			Name:    name,
			Rows:    nullInt(rowCount),
			Size:    nullInt(size),
			Columns: nullInt(columns),
			Comment: comment,
		})
	}
	return out, rows.Err()
}

func nullInt(value stdsql.NullInt64) int64 {
	if !value.Valid {
		return -1
	}
	return value.Int64
}
