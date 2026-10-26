// Package ddl builds schema-changing statements for the SQL dialects dbrix
// supports. Statements are plain text so the UI can preview exactly what will
// run before executing it.
package ddl

import (
	"strings"

	"github.com/LinduanTof/dbrix/internal/db/dialect"
)

func dialectFor(typ string) dialect.Dialect {
	switch typ {
	case "mysql":
		return dialect.MySQL{}
	case "sqlite":
		return dialect.SQLite{}
	default:
		return dialect.Postgres{}
	}
}

func quoteList(d dialect.Dialect, values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = d.Quote(value)
	}
	return strings.Join(quoted, ", ")
}

func DropTable(typ, schema, table string) string {
	d := dialectFor(typ)
	return "DROP TABLE " + d.Qualified(schema, table) + ";"
}

func TruncateTable(typ, schema, table string) string {
	d := dialectFor(typ)
	if typ == "sqlite" {
		return "DELETE FROM " + d.Qualified(schema, table) + ";"
	}
	return "TRUNCATE TABLE " + d.Qualified(schema, table) + ";"
}

func RenameTable(typ, schema, table, newName string) string {
	d := dialectFor(typ)
	return "ALTER TABLE " + d.Qualified(schema, table) + " RENAME TO " + d.Quote(newName) + ";"
}

func AddColumn(typ, schema, table, column, dataType string, nullable bool, defaultExpr string) string {
	d := dialectFor(typ)
	var b strings.Builder
	b.WriteString("ALTER TABLE ")
	b.WriteString(d.Qualified(schema, table))
	b.WriteString(" ADD COLUMN ")
	b.WriteString(d.Quote(column))
	b.WriteString(" ")
	b.WriteString(strings.TrimSpace(dataType))
	if !nullable {
		b.WriteString(" NOT NULL")
	}
	if expr := strings.TrimSpace(defaultExpr); expr != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(expr)
	}
	b.WriteString(";")
	return b.String()
}

func DropColumn(typ, schema, table, column string) string {
	d := dialectFor(typ)
	return "ALTER TABLE " + d.Qualified(schema, table) + " DROP COLUMN " + d.Quote(column) + ";"
}

func CreateIndex(typ, schema, table, name string, columns []string, unique bool) string {
	d := dialectFor(typ)
	prefix := "CREATE INDEX "
	if unique {
		prefix = "CREATE UNIQUE INDEX "
	}
	return prefix + d.Quote(name) + " ON " + d.Qualified(schema, table) +
		" (" + quoteList(d, columns) + ");"
}

func DropIndex(typ, schema, table, name string) string {
	d := dialectFor(typ)
	switch typ {
	case "mysql":
		return "DROP INDEX " + d.Quote(name) + " ON " + d.Qualified(schema, table) + ";"
	case "sqlite":
		return "DROP INDEX " + d.Quote(name) + ";"
	default:
		return "DROP INDEX " + d.Qualified(schema, name) + ";"
	}
}

// CreateTable wraps a user-supplied column definition body.
func CreateTable(typ, schema, table, body string) string {
	d := dialectFor(typ)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i, line := range lines {
		lines[i] = "  " + strings.TrimSuffix(strings.TrimSpace(line), ",")
	}
	return "CREATE TABLE " + d.Qualified(schema, table) + " (\n" + strings.Join(lines, ",\n") + "\n);"
}
