package export

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/db/dialect"
)

// CSVStream writes rows as they are read from the cursor.
func CSVStream(w io.Writer, stream db.RowStream) error {
	cw := csv.NewWriter(w)
	columns := stream.Columns()
	header := make([]string, len(columns))
	for i, column := range columns {
		header[i] = column.Name
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for stream.Next() {
		row, err := stream.Values()
		if err != nil {
			return err
		}
		record := make([]string, len(row))
		for i, value := range row {
			record[i] = csvValue(value)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := stream.Err(); err != nil {
		return err
	}
	return cw.Error()
}

// JSONStream writes a streaming JSON array of row objects.
func JSONStream(w io.Writer, stream db.RowStream) error {
	bw := bufio.NewWriterSize(w, 64*1024)
	bw.WriteString("[")
	first := true
	for stream.Next() {
		row, err := stream.Values()
		if err != nil {
			return err
		}
		if !first {
			bw.WriteString(",")
		}
		bw.WriteString("\n  ")
		if err := writeJSONObject(bw, stream.Columns(), row); err != nil {
			return err
		}
		first = false
	}
	if !first {
		bw.WriteString("\n")
	}
	bw.WriteString("]\n")
	if err := stream.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// SQLTableStream writes the DDL and one INSERT per streamed row.
func SQLTableStream(w io.Writer, d dialect.Dialect, schema, table, ddl string, stream db.RowStream) error {
	qualified := d.Qualified(schema, table)
	if _, err := fmt.Fprintf(w, "-- %s\n", qualified); err != nil {
		return err
	}
	if err := SQLStructure(w, ddl); err != nil {
		return err
	}
	bw := bufio.NewWriterSize(w, 64*1024)
	columns := make([]string, len(stream.Columns()))
	for i, column := range stream.Columns() {
		columns[i] = d.Quote(column.Name)
	}
	prefix := "INSERT INTO " + qualified + " (" + strings.Join(columns, ", ") + ") VALUES ("
	for stream.Next() {
		row, err := stream.Values()
		if err != nil {
			return err
		}
		values := make([]string, len(columns))
		for i := range columns {
			if i < len(row) {
				values[i] = sqlLiteral(d, row[i])
			} else {
				values[i] = "NULL"
			}
		}
		bw.WriteString(prefix + strings.Join(values, ", ") + ");\n")
	}
	if err := stream.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// TableStream describes a table whose rows are fetched lazily.
type TableStream struct {
	Schema string
	Table  string
	Open   func() (db.RowStream, error)
	Done   func()
}

// JSONTablesStream writes one JSON object whose keys are qualified table
// names, opening each table cursor in turn.
func JSONTablesStream(w io.Writer, tables []TableStream) error {
	bw := bufio.NewWriterSize(w, 64*1024)
	bw.WriteString("{\n")
	for i, table := range tables {
		key := table.Table
		if table.Schema != "" && table.Schema != "main" {
			key = table.Schema + "." + table.Table
		}
		encoded, err := json.Marshal(key)
		if err != nil {
			return err
		}
		bw.WriteString("  ")
		bw.Write(encoded)
		bw.WriteString(": ")
		stream, err := table.Open()
		if err != nil {
			return err
		}
		err = writeJSONArrayStream(bw, stream)
		if closeErr := stream.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if table.Done != nil {
			table.Done()
		}
		if i < len(tables)-1 {
			bw.WriteString(",")
		}
		bw.WriteString("\n")
	}
	bw.WriteString("}\n")
	return bw.Flush()
}

func writeJSONArrayStream(bw *bufio.Writer, stream db.RowStream) error {
	bw.WriteString("[")
	first := true
	for stream.Next() {
		row, err := stream.Values()
		if err != nil {
			return err
		}
		if !first {
			bw.WriteString(", ")
		}
		if err := writeJSONObject(bw, stream.Columns(), row); err != nil {
			return err
		}
		first = false
	}
	bw.WriteString("]")
	return stream.Err()
}

func writeJSONObject(bw *bufio.Writer, columns []db.ColumnMeta, row []any) error {
	bw.WriteString("{")
	for j, column := range columns {
		if j > 0 {
			bw.WriteString(", ")
		}
		key, err := json.Marshal(column.Name)
		if err != nil {
			return err
		}
		var value any
		if j < len(row) {
			value = row[j]
		}
		encoded, err := json.Marshal(jsonValue(value))
		if err != nil {
			return err
		}
		bw.Write(key)
		bw.WriteString(": ")
		bw.Write(encoded)
	}
	bw.WriteString("}")
	return nil
}
