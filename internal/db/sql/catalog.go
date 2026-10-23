package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"strings"

	"github.com/LinduanTof/b-db/internal/db"
)

type catalog struct{ c *conn }

func (k catalog) Schemas(ctx context.Context) ([]string, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgSchemas(ctx)
	case "mysql":
		return k.mysqlSchemas(ctx)
	case "sqlite":
		return []string{"main"}, nil
	default:
		return nil, fmt.Errorf("catalog not implemented for %s", k.c.typ)
	}
}

func (k catalog) Tables(ctx context.Context, schema string) ([]db.Object, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgTables(ctx, schema)
	case "mysql":
		return k.mysqlTables(ctx, schema)
	case "sqlite":
		return k.sqliteTables(ctx)
	default:
		return nil, fmt.Errorf("catalog not implemented for %s", k.c.typ)
	}
}

func (k catalog) Columns(ctx context.Context, schema, table string) ([]db.Column, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgColumns(ctx, schema, table)
	case "mysql":
		return k.mysqlColumns(ctx, schema, table)
	case "sqlite":
		return k.sqliteColumns(ctx, table)
	default:
		return nil, fmt.Errorf("catalog not implemented for %s", k.c.typ)
	}
}

func (k catalog) Indexes(ctx context.Context, schema, table string) ([]db.Index, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgIndexes(ctx, schema, table)
	case "mysql":
		return k.mysqlIndexes(ctx, schema, table)
	case "sqlite":
		return k.sqliteIndexes(ctx, table)
	default:
		return nil, fmt.Errorf("catalog not implemented for %s", k.c.typ)
	}
}

func (k catalog) ForeignKeys(ctx context.Context, schema, table string) ([]db.ForeignKey, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgForeignKeys(ctx, schema, table)
	case "mysql":
		return k.mysqlForeignKeys(ctx, schema, table)
	case "sqlite":
		return k.sqliteForeignKeys(ctx, table)
	default:
		return nil, fmt.Errorf("catalog not implemented for %s", k.c.typ)
	}
}

func (k catalog) DDL(ctx context.Context, schema, table string) (string, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgDDL(ctx, schema, table)
	case "mysql":
		return k.mysqlDDL(ctx, schema, table)
	case "sqlite":
		return k.sqliteDDL(ctx, table)
	default:
		return "", fmt.Errorf("DDL not implemented for %s", k.c.typ)
	}
}

func (k catalog) Databases(ctx context.Context) ([]string, error) {
	switch k.c.typ {
	case "postgres":
		return k.pgDatabases(ctx)
	default:
		return nil, nil
	}
}

func (k catalog) pgDatabases(ctx context.Context) ([]string, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT datname
		FROM pg_database
		WHERE datallowconn AND NOT datistemplate
		  AND has_database_privilege(datname, 'CONNECT')
		ORDER BY datname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (k catalog) pgSchemas(ctx context.Context) ([]string, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT nspname
		FROM pg_namespace
		WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'
		ORDER BY nspname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (k catalog) pgTables(ctx context.Context, schema string) ([]db.Object, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY table_name`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Object
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return nil, err
		}
		kind := "table"
		if tableType == "VIEW" {
			kind = "view"
		}
		out = append(out, db.Object{Schema: schema, Name: name, Kind: kind})
	}
	return out, rows.Err()
}

func (k catalog) pgColumns(ctx context.Context, schema, table string) ([]db.Column, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT c.column_name,
		       c.data_type,
		       (c.is_nullable = 'YES'),
		       COALESCE(c.column_default, ''),
		       EXISTS (
		           SELECT 1
		           FROM information_schema.table_constraints tc
		           JOIN information_schema.key_column_usage kcu
		             ON kcu.constraint_name = tc.constraint_name
		            AND kcu.constraint_schema = tc.constraint_schema
		            AND kcu.table_name = tc.table_name
		            AND kcu.table_schema = tc.table_schema
		           WHERE tc.constraint_type = 'PRIMARY KEY'
		             AND tc.table_schema = c.table_schema
		             AND tc.table_name = c.table_name
		             AND kcu.column_name = c.column_name
		       )
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Column
	for rows.Next() {
		var col db.Column
		if err := rows.Scan(&col.Name, &col.DataType, &col.Nullable, &col.Default, &col.PrimaryKey); err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, rows.Err()
}

func (k catalog) pgIndexes(ctx context.Context, schema, table string) ([]db.Index, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT i.relname, ix.indisunique, a.attname
		FROM pg_index ix
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname, k.ord`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []db.Index
	index := map[string]int{}
	for rows.Next() {
		var name, column string
		var unique bool
		if err := rows.Scan(&name, &unique, &column); err != nil {
			return nil, err
		}
		idx, ok := index[name]
		if !ok {
			idx = len(out)
			index[name] = idx
			out = append(out, db.Index{Name: name, Unique: unique})
		}
		out[idx].Columns = append(out[idx].Columns, column)
	}
	return out, rows.Err()
}

func (k catalog) pgForeignKeys(ctx context.Context, schema, table string) ([]db.ForeignKey, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT con.conname, a.attname, rt.relname, ra.attname
		FROM pg_constraint con
		JOIN pg_class t ON t.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_class rt ON rt.oid = con.confrelid
		JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS ck(attnum, ord) ON true
		JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS fk(attnum, ord) ON fk.ord = ck.ord
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ck.attnum
		JOIN pg_attribute ra ON ra.attrelid = rt.oid AND ra.attnum = fk.attnum
		WHERE con.contype = 'f' AND n.nspname = $1 AND t.relname = $2
		ORDER BY con.conname, ck.ord`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.ForeignKey
	for rows.Next() {
		var fk db.ForeignKey
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.RefTable, &fk.RefColumn); err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, rows.Err()
}

func (k catalog) pgDDL(ctx context.Context, schema, table string) (string, error) {
	var kind string
	var viewDef stdsql.NullString
	err := k.c.db.QueryRowContext(ctx, `
		SELECT c.relkind::text, pg_get_viewdef(c.oid, true)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2`, schema, table).Scan(&kind, &viewDef)
	qualified := k.c.d.Qualified(schema, table)
	if err == nil && (kind == "v" || kind == "m") && viewDef.Valid {
		def := strings.TrimSpace(viewDef.String)
		if !strings.HasSuffix(def, ";") {
			def += ";"
		}
		return fmt.Sprintf("CREATE VIEW %s AS\n%s", qualified, def), nil
	}

	columns, err := k.pgColumns(ctx, schema, table)
	if err != nil {
		return "", err
	}
	indexes, err := k.pgIndexes(ctx, schema, table)
	if err != nil {
		return "", err
	}
	fks, err := k.pgForeignKeys(ctx, schema, table)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", qualified)
	defs := make([]string, 0, len(columns)+1)
	pkRaw := map[string]bool{}
	for _, col := range columns {
		def := "    " + k.c.d.Quote(col.Name) + " " + col.DataType
		if !col.Nullable {
			def += " NOT NULL"
		}
		if col.Default != "" {
			def += " DEFAULT " + col.Default
		}
		defs = append(defs, def)
		if col.PrimaryKey {
			pkRaw[col.Name] = true
		}
	}
	if len(pkRaw) > 0 {
		pks := make([]string, 0, len(pkRaw))
		for _, col := range columns {
			if col.PrimaryKey {
				pks = append(pks, k.c.d.Quote(col.Name))
			}
		}
		defs = append(defs, "    PRIMARY KEY ("+strings.Join(pks, ", ")+")")
	}
	b.WriteString(strings.Join(defs, ",\n"))
	b.WriteString("\n);\n")

	for _, idx := range indexes {
		if idx.Unique && len(idx.Columns) == len(pkRaw) {
			allPK := true
			for _, col := range idx.Columns {
				if !pkRaw[col] {
					allPK = false
					break
				}
			}
			if allPK {
				continue
			}
		}
		unique := ""
		if idx.Unique {
			unique = "UNIQUE "
		}
		cols := make([]string, len(idx.Columns))
		for i, col := range idx.Columns {
			cols[i] = k.c.d.Quote(col)
		}
		fmt.Fprintf(&b, "CREATE %sINDEX %s ON %s (%s);\n", unique, k.c.d.Quote(idx.Name), qualified, strings.Join(cols, ", "))
	}
	for _, fk := range fks {
		fmt.Fprintf(&b, "ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s);\n",
			qualified, k.c.d.Quote(fk.Name), k.c.d.Quote(fk.Column), k.c.d.Quote(fk.RefTable), k.c.d.Quote(fk.RefColumn))
	}
	return strings.TrimSpace(b.String()), nil
}

func (k catalog) mysqlSchemas(ctx context.Context) ([]string, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
		ORDER BY schema_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (k catalog) mysqlTables(ctx context.Context, schema string) ([]db.Object, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY table_name`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Object
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return nil, err
		}
		kind := "table"
		if tableType == "VIEW" {
			kind = "view"
		}
		out = append(out, db.Object{Schema: schema, Name: name, Kind: kind})
	}
	return out, rows.Err()
}

func (k catalog) mysqlColumns(ctx context.Context, schema, table string) ([]db.Column, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT column_name,
		       column_type,
		       (is_nullable = 'YES'),
		       COALESCE(column_default, ''),
		       (column_key = 'PRI')
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Column
	for rows.Next() {
		var col db.Column
		if err := rows.Scan(&col.Name, &col.DataType, &col.Nullable, &col.Default, &col.PrimaryKey); err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, rows.Err()
}

func (k catalog) mysqlIndexes(ctx context.Context, schema, table string) ([]db.Index, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT index_name, (non_unique = 0), column_name
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		ORDER BY index_name, seq_in_index`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []db.Index
	index := map[string]int{}
	for rows.Next() {
		var name, column string
		var unique bool
		if err := rows.Scan(&name, &unique, &column); err != nil {
			return nil, err
		}
		idx, ok := index[name]
		if !ok {
			idx = len(out)
			index[name] = idx
			out = append(out, db.Index{Name: name, Unique: unique})
		}
		out[idx].Columns = append(out[idx].Columns, column)
	}
	return out, rows.Err()
}

func (k catalog) mysqlForeignKeys(ctx context.Context, schema, table string) ([]db.ForeignKey, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT constraint_name, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND table_name = ? AND referenced_table_name IS NOT NULL
		ORDER BY constraint_name, ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.ForeignKey
	for rows.Next() {
		var fk db.ForeignKey
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.RefTable, &fk.RefColumn); err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, rows.Err()
}

func (k catalog) mysqlDDL(ctx context.Context, schema, table string) (string, error) {
	qualified := k.c.d.Qualified(schema, table)
	var lastErr error
	for _, stmt := range []string{
		"SHOW CREATE TABLE " + qualified,
		"SHOW CREATE VIEW " + qualified,
	} {
		rows, err := k.c.db.QueryContext(ctx, stmt)
		if err != nil {
			lastErr = err
			continue
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			lastErr = err
			continue
		}
		if rows.Next() {
			values := make([]any, len(columns))
			ptrs := make([]any, len(columns))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return "", err
			}
			rows.Close()
			if len(values) >= 2 {
				switch v := values[1].(type) {
				case []byte:
					return string(v), nil
				case string:
					return v, nil
				default:
					return fmt.Sprint(v), nil
				}
			}
		}
		rows.Close()
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no DDL found for %s", qualified)
}

func (k catalog) sqliteDDL(ctx context.Context, table string) (string, error) {
	var ddl stdsql.NullString
	err := k.c.db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE name = ? AND type IN ('table', 'view')`, table).Scan(&ddl)
	if err != nil {
		return "", err
	}
	if !ddl.Valid {
		return "", fmt.Errorf("no DDL found for %s", table)
	}

	var b strings.Builder
	text := strings.TrimSpace(ddl.String)
	if !strings.HasSuffix(text, ";") {
		text += ";"
	}
	b.WriteString(text)

	rows, err := k.c.db.QueryContext(ctx,
		`SELECT sql FROM sqlite_master WHERE tbl_name = ? AND type = 'index' AND sql IS NOT NULL ORDER BY name`, table)
	if err != nil {
		return b.String(), nil
	}
	defer rows.Close()
	for rows.Next() {
		var indexDDL string
		if err := rows.Scan(&indexDDL); err != nil {
			return b.String(), nil
		}
		b.WriteString("\n")
		text := strings.TrimSpace(indexDDL)
		if !strings.HasSuffix(text, ";") {
			text += ";"
		}
		b.WriteString(text)
	}
	if err := rows.Err(); err != nil {
		return b.String(), nil
	}
	return b.String(), nil
}

func (k catalog) sqliteTables(ctx context.Context) ([]db.Object, error) {
	rows, err := k.c.db.QueryContext(ctx, `
		SELECT name, type
		FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Object
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		out = append(out, db.Object{Schema: "main", Name: name, Kind: typ})
	}
	return out, rows.Err()
}

func (k catalog) sqliteColumns(ctx context.Context, table string) ([]db.Column, error) {
	rows, err := k.c.db.QueryContext(ctx, `PRAGMA table_info(`+k.c.d.Quote(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.Column
	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   stdsql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultV, &primaryKey); err != nil {
			return nil, err
		}
		out = append(out, db.Column{
			Name:       name,
			DataType:   dataType,
			Nullable:   notNull == 0 && primaryKey == 0,
			Default:    defaultV.String,
			PrimaryKey: primaryKey > 0,
		})
	}
	return out, rows.Err()
}

func (k catalog) sqliteIndexes(ctx context.Context, table string) ([]db.Index, error) {
	rows, err := k.c.db.QueryContext(ctx, `PRAGMA index_list(`+k.c.d.Quote(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawIndex struct {
		name   string
		unique bool
	}
	var raw []rawIndex
	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		raw = append(raw, rawIndex{name: name, unique: unique != 0})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []db.Index
	for _, r := range raw {
		cols, err := k.sqliteIndexColumns(ctx, r.name)
		if err != nil {
			return nil, err
		}
		out = append(out, db.Index{Name: r.name, Columns: cols, Unique: r.unique})
	}
	return out, nil
}

func (k catalog) sqliteIndexColumns(ctx context.Context, index string) ([]string, error) {
	rows, err := k.c.db.QueryContext(ctx, `PRAGMA index_info(`+k.c.d.Quote(index)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var (
			seqno int
			cid   int
			name  stdsql.NullString
		)
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			cols = append(cols, name.String)
		}
	}
	return cols, rows.Err()
}

func (k catalog) sqliteForeignKeys(ctx context.Context, table string) ([]db.ForeignKey, error) {
	rows, err := k.c.db.QueryContext(ctx, `PRAGMA foreign_key_list(`+k.c.d.Quote(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.ForeignKey
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			from     stdsql.NullString
			to       stdsql.NullString
			onUpdate string
			onDelete string
			match    string
		)
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		out = append(out, db.ForeignKey{
			Name:      fmt.Sprintf("fk_%d", id),
			Column:    from.String,
			RefTable:  refTable,
			RefColumn: to.String,
		})
	}
	return out, rows.Err()
}
