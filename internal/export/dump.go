package export

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/LinduanTof/dbrix/internal/db"
	"github.com/LinduanTof/dbrix/internal/db/dialect"
)

// SQLStructure writes a single DDL statement with a trailing semicolon.
func SQLStructure(w io.Writer, ddl string) error {
	ddl = strings.TrimSpace(ddl)
	if ddl == "" {
		return nil
	}
	if !strings.HasSuffix(ddl, ";") {
		ddl += ";"
	}
	_, err := io.WriteString(w, ddl+"\n")
	return err
}

// SQLTable writes the DDL followed by one INSERT statement per row.
func SQLTable(w io.Writer, d dialect.Dialect, schema, table, ddl string, rs *db.ResultSet) error {
	qualified := d.Qualified(schema, table)
	if _, err := fmt.Fprintf(w, "-- %s\n", qualified); err != nil {
		return err
	}
	if err := SQLStructure(w, ddl); err != nil {
		return err
	}
	if rs == nil || len(rs.Rows) == 0 {
		return nil
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	columns := make([]string, len(rs.Columns))
	for i, column := range rs.Columns {
		columns[i] = d.Quote(column.Name)
	}
	prefix := "INSERT INTO " + qualified + " (" + strings.Join(columns, ", ") + ") VALUES ("
	for _, row := range rs.Rows {
		values := make([]string, len(rs.Columns))
		for i := range rs.Columns {
			if i < len(row) {
				values[i] = sqlLiteral(d, row[i])
			} else {
				values[i] = "NULL"
			}
		}
		if _, err := io.WriteString(w, prefix+strings.Join(values, ", ")+");\n"); err != nil {
			return err
		}
	}
	return nil
}

func sqlLiteral(d dialect.Dialect, value any) string {
	switch x := value.(type) {
	case nil:
		return "NULL"
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(x)
	case time.Time:
		return quoteString(d, x.Format("2006-01-02 15:04:05.999999"))
	case []byte:
		encoded := hex.EncodeToString(x)
		if _, ok := d.(dialect.Postgres); ok {
			return "decode('" + encoded + "', 'hex')"
		}
		return "X'" + encoded + "'"
	case string:
		return quoteString(d, x)
	default:
		return quoteString(d, fmt.Sprint(x))
	}
}

func quoteString(d dialect.Dialect, value string) string {
	value = strings.ReplaceAll(value, "'", "''")
	if _, ok := d.(dialect.MySQL); ok {
		value = strings.ReplaceAll(value, `\`, `\\`)
	}
	return "'" + value + "'"
}

// TableData pairs a result set with the table it came from.
type TableData struct {
	Schema string
	Table  string
	Result *db.ResultSet
}

// JSONTables writes one JSON object whose keys are qualified table names.
func JSONTables(w io.Writer, tables []TableData) error {
	var b strings.Builder
	b.WriteString("{\n")
	for i, table := range tables {
		key := table.Table
		if table.Schema != "" && table.Schema != "main" {
			key = table.Schema + "." + table.Table
		}
		encoded, err := json.Marshal(key)
		if err != nil {
			return err
		}
		b.WriteString("  ")
		b.Write(encoded)
		b.WriteString(": ")
		rows, err := jsonRows(table.Result)
		if err != nil {
			return err
		}
		b.WriteString(rows)
		if i < len(tables)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func jsonRows(rs *db.ResultSet) (string, error) {
	if rs == nil || len(rs.Rows) == 0 {
		return "[]", nil
	}
	var b strings.Builder
	b.WriteString("[")
	for i, row := range rs.Rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{")
		for j, column := range rs.Columns {
			if j > 0 {
				b.WriteString(", ")
			}
			key, err := json.Marshal(column.Name)
			if err != nil {
				return "", err
			}
			var value any
			if j < len(row) {
				value = row[j]
			}
			encoded, err := json.Marshal(jsonValue(value))
			if err != nil {
				return "", err
			}
			b.Write(key)
			b.WriteString(": ")
			b.Write(encoded)
		}
		b.WriteString("}")
	}
	b.WriteString("]")
	return b.String(), nil
}
