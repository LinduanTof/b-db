package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LinduanTof/b-db/internal/db"
)

func CSV(w io.Writer, rs *db.ResultSet) error {
	cw := csv.NewWriter(w)
	header := make([]string, len(rs.Columns))
	for i, col := range rs.Columns {
		header[i] = col.Name
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rs.Rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = csvValue(v)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func JSON(w io.Writer, rs *db.ResultSet) error {
	if len(rs.Rows) == 0 {
		_, err := io.WriteString(w, "[]\n")
		return err
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, row := range rs.Rows {
		b.WriteString("  {")
		for j, col := range rs.Columns {
			if j > 0 {
				b.WriteString(", ")
			}
			key, err := json.Marshal(col.Name)
			if err != nil {
				return err
			}
			value, err := json.Marshal(jsonValue(row[j]))
			if err != nil {
				return err
			}
			b.Write(key)
			b.WriteString(": ")
			b.Write(value)
		}
		b.WriteString("}")
		if i < len(rs.Rows)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func Markdown(w io.Writer, rs *db.ResultSet) error {
	var b strings.Builder
	b.WriteString("|")
	for _, col := range rs.Columns {
		b.WriteString(" ")
		b.WriteString(markdownCell(col.Name))
		b.WriteString(" |")
	}
	b.WriteString("\n|")
	for range rs.Columns {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, row := range rs.Rows {
		b.WriteString("|")
		for i := range rs.Columns {
			text := ""
			if i < len(row) {
				text = csvValue(row[i])
			}
			b.WriteString(" ")
			b.WriteString(markdownCell(text))
			b.WriteString(" |")
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func markdownCell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "\r", "<br>")
	return s
}

func csvValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case bool:
		return strconv.FormatBool(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func jsonValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		return x
	default:
		return v
	}
}
