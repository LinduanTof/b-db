package sql

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/linduan-guan/b-db/internal/db"
)

type mutator struct{ c *conn }

func (m mutator) UpdateCell(ctx context.Context, table db.TableRef, key map[string]any, column string, value any) (int64, error) {
	if len(key) == 0 {
		return 0, fmt.Errorf("refusing to update without a primary key")
	}
	query, args := m.updateSQL(table, key, column, value)
	return m.execOne(ctx, query, args...)
}

func (m mutator) DeleteRow(ctx context.Context, table db.TableRef, key map[string]any) (int64, error) {
	if len(key) == 0 {
		return 0, fmt.Errorf("refusing to delete without a primary key")
	}
	query, args := m.deleteSQL(table, key)
	return m.execOne(ctx, query, args...)
}

func (m mutator) PreviewUpdate(table db.TableRef, key map[string]any, column string, value any) string {
	return fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s;",
		m.c.d.Qualified(table.Schema, table.Name),
		m.c.d.Quote(column),
		literal(value),
		previewWhere(m.c.d, key),
	)
}

func (m mutator) PreviewDelete(table db.TableRef, key map[string]any) string {
	return fmt.Sprintf("DELETE FROM %s WHERE %s;",
		m.c.d.Qualified(table.Schema, table.Name),
		previewWhere(m.c.d, key),
	)
}

func (m mutator) execOne(ctx context.Context, query string, args ...any) (int64, error) {
	tx, err := m.c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	if affected != 1 {
		tx.Rollback()
		return affected, fmt.Errorf("expected exactly 1 row to change, got %d", affected)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}

func (m mutator) updateSQL(table db.TableRef, key map[string]any, column string, value any) (string, []any) {
	args := []any{value}
	where, whereArgs := m.where(key, 2)
	args = append(args, whereArgs...)
	query := fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s",
		m.c.d.Qualified(table.Schema, table.Name),
		m.c.d.Quote(column),
		m.c.d.Placeholder(1),
		where,
	)
	return query, args
}

func (m mutator) deleteSQL(table db.TableRef, key map[string]any) (string, []any) {
	where, args := m.where(key, 1)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s",
		m.c.d.Qualified(table.Schema, table.Name),
		where,
	)
	return query, args
}

func (m mutator) where(key map[string]any, start int) (string, []any) {
	keys := make([]string, 0, len(key))
	for k := range key {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for i, k := range keys {
		parts = append(parts, m.c.d.Quote(k)+" = "+m.c.d.Placeholder(start+i))
		args = append(args, key[k])
	}
	return strings.Join(parts, " AND "), args
}

func previewWhere(d interface {
	Quote(string) string
}, key map[string]any) string {
	keys := make([]string, 0, len(key))
	for k := range key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, d.Quote(k)+" = "+literal(key[k]))
	}
	return strings.Join(parts, " AND ")
}

func literal(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case []byte:
		return "'" + strings.ReplaceAll(string(x), "'", "''") + "'"
	default:
		return fmt.Sprint(x)
	}
}
