package redis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LinduanTof/b-db/internal/db"
)

func splitCommand(input string) ([]string, error) {
	var out []string
	var current strings.Builder
	var quote rune
	escaped := false
	started := false

	flush := func() {
		if started {
			out = append(out, current.String())
			current.Reset()
			started = false
		}
	}

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			started = true
			escaped = false
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			if r == '\\' && quote == '"' {
				escaped = true
				continue
			}
			current.WriteRune(r)
			started = true
			continue
		}
		switch r {
		case '\\':
			escaped = true
			started = true
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("trailing escape in command")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	flush()
	return out, nil
}

func toArgs(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func convert(command string, value any, maxRows int) *db.ResultSet {
	name := strings.ToUpper(command)
	switch v := value.(type) {
	case nil:
		return &db.ResultSet{}
	case []any:
		if name == "HGETALL" || name == "CONFIG" {
			return pairResult(v, maxRows)
		}
		rows := make([][]any, 0, len(v))
		for _, item := range v {
			if maxRows > 0 && len(rows) >= maxRows {
				break
			}
			rows = append(rows, []any{formatRedisValue(item)})
		}
		return &db.ResultSet{
			Columns:   []db.ColumnMeta{{Name: "value"}},
			Rows:      rows,
			Truncated: maxRows > 0 && len(v) > maxRows,
		}
	case map[string]any:
		pairs := make([]any, 0, len(v)*2)
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			pairs = append(pairs, key, v[key])
		}
		return pairResult(pairs, maxRows)
	case map[string]string:
		pairs := make([]any, 0, len(v)*2)
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			pairs = append(pairs, key, v[key])
		}
		return pairResult(pairs, maxRows)
	default:
		return &db.ResultSet{
			Columns: []db.ColumnMeta{{Name: "value"}},
			Rows:    [][]any{{formatRedisValue(value)}},
		}
	}
}

func pairResult(values []any, maxRows int) *db.ResultSet {
	rows := make([][]any, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		if maxRows > 0 && len(rows) >= maxRows {
			break
		}
		rows = append(rows, []any{formatRedisValue(values[i]), formatRedisValue(values[i+1])})
	}
	return &db.ResultSet{
		Columns:   []db.ColumnMeta{{Name: "field"}, {Name: "value"}},
		Rows:      rows,
		Truncated: maxRows > 0 && len(values)/2 > maxRows,
	}
}

func formatRedisValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return v
	case []byte:
		return string(v)
	case int64, float64, bool:
		return v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(encoded)
	}
}
