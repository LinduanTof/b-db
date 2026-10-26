// Package importer reads rows from CSV and JSON files for the import flow.
// CSV is streamed, so file size does not affect memory use; JSON is decoded
// fully because object keys must be known before the first row is emitted.
package importer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Format string

const (
	FormatCSV  Format = "csv"
	FormatJSON Format = "json"
)

// Reader yields rows parsed from a file. Next returns io.EOF at the end.
type Reader interface {
	// Columns returns the column names declared by the file. It is empty for
	// CSV files opened without a header row.
	Columns() []string
	Next() ([]any, error)
	Close() error
}

func Open(path string, format Format, header bool) (Reader, error) {
	switch format {
	case FormatJSON:
		return openJSON(path)
	default:
		return openCSV(path, header)
	}
}

type csvReader struct {
	file    *os.File
	reader  *csv.Reader
	columns []string
}

func openCSV(path string, header bool) (Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	delimiter, err := detectDelimiter(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}
	reader := csv.NewReader(file)
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	out := &csvReader{file: file, reader: reader}
	if header {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			file.Close()
			return nil, err
		}
		out.columns = append([]string(nil), record...)
	}
	return out, nil
}

// detectDelimiter picks the separator used most often outside quotes on the
// first line. Comma remains the default when the line is ambiguous.
func detectDelimiter(file *os.File) (rune, error) {
	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return ',', err
	}
	line := string(buf[:n])
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	counts := map[rune]int{',': 0, ';': 0, '\t': 0}
	inQuotes := false
	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		default:
			if !inQuotes {
				if _, ok := counts[r]; ok {
					counts[r]++
				}
			}
		}
	}
	best := ','
	for _, candidate := range []rune{';', '\t'} {
		if counts[candidate] > counts[best] {
			best = candidate
		}
	}
	return best, nil
}

func (r *csvReader) Columns() []string { return r.columns }

func (r *csvReader) Next() ([]any, error) {
	record, err := r.reader.Read()
	if err != nil {
		return nil, err
	}
	row := make([]any, len(record))
	for i, value := range record {
		row[i] = value
	}
	return row, nil
}

func (r *csvReader) Close() error { return r.file.Close() }

type jsonReader struct {
	columns []string
	rows    [][]any
	index   int
}

func openJSON(path string) (Reader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("expected a JSON array of objects: %w", err)
	}
	seen := map[string]bool{}
	for _, record := range records {
		for key := range record {
			seen[key] = true
		}
	}
	columns := make([]string, 0, len(seen))
	for key := range seen {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	out := &jsonReader{columns: columns, rows: make([][]any, len(records))}
	for i, record := range records {
		row := make([]any, len(columns))
		for j, column := range columns {
			if value, ok := record[column]; ok {
				row[j] = value
			}
		}
		out.rows[i] = row
	}
	return out, nil
}

func (r *jsonReader) Columns() []string { return r.columns }

func (r *jsonReader) Next() ([]any, error) {
	if r.index >= len(r.rows) {
		return nil, io.EOF
	}
	row := r.rows[r.index]
	r.index++
	return row, nil
}

func (r *jsonReader) Close() error { return nil }
