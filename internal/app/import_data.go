package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/dbrix/internal/db"
	"github.com/LinduanTof/dbrix/internal/i18n"
	"github.com/LinduanTof/dbrix/internal/importer"
	"github.com/LinduanTof/dbrix/internal/ui/form"
)

const (
	importPreviewRows  = 5
	importProgressStep = 1000
	importTimeout      = 30 * time.Minute
)

type importPlan struct {
	path    string
	format  importer.Format
	header  bool
	table   db.TableRef
	columns []db.Column // target columns, in file order
	indexes []int       // position of each target column in a source row
}

type importPreviewMsg struct {
	id     uint64
	plan   importPlan
	rows   [][]any
	insert string
	err    error
}

type importProgressMsg struct {
	label string
	rows  int64
}

type importedMsg struct {
	id    uint64
	table db.TableRef
	rows  int64
	err   error
}

func (m *Model) openImportForm(schema, table string) tea.Cmd {
	if m.readOnly() {
		m.setStatus(m.t.T(i18n.KeyErrorReadOnly), true)
		return nil
	}
	m.formTarget = db.TableRef{Schema: schema, Name: table}
	title := fmt.Sprintf(m.t.T(i18n.KeyImportTitle), qualifiedTable(schema, table))
	m.form.Open("import", title, m.t.T(i18n.KeyImportNote), []form.Field{
		{Label: m.t.T(i18n.KeyImportFile), Placeholder: "data.csv"},
		{Label: m.t.T(i18n.KeyImportFormat), Choices: []string{
			m.t.T(i18n.KeyImportFormatCSV), m.t.T(i18n.KeyImportFormatJSON),
		}},
		{Label: m.t.T(i18n.KeyImportHeader), Choices: []string{
			m.t.T(i18n.KeyImportYes), m.t.T(i18n.KeyImportNo),
		}},
	})
	return nil
}

func (m *Model) submitImportForm(values []string, choices []int) tea.Cmd {
	path := ""
	if len(values) > 0 {
		path = strings.TrimSpace(values[0])
	}
	if path == "" {
		m.form.SetError(m.t.T(i18n.KeyErrorImportFile))
		return nil
	}
	format := importer.FormatCSV
	if len(choices) > 1 && choices[1] == 1 {
		format = importer.FormatJSON
	}
	header := len(choices) < 3 || choices[2] == 0
	target := m.formTarget
	columns := m.formColumns
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	if _, ok := conn.Mutator().(db.Inserter); !ok {
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, conn.Type()), true)
		return nil
	}
	id := m.connectSeq
	return func() tea.Msg {
		plan := importPlan{path: path, format: format, header: header, table: target}
		reader, err := importer.Open(path, format, header)
		if err != nil {
			return importPreviewMsg{id: id, err: err}
		}
		defer reader.Close()
		source := reader.Columns()
		var rows [][]any
		for len(rows) < importPreviewRows {
			row, err := reader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return importPreviewMsg{id: id, err: err}
			}
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			return importPreviewMsg{id: id, err: errors.New("import_empty")}
		}
		indexes, mapped, err := matchImportColumns(columns, source, len(rows[0]))
		if err != nil {
			return importPreviewMsg{id: id, err: err}
		}
		plan.indexes, plan.columns = indexes, mapped
		preview := make([][]any, len(rows))
		for i, row := range rows {
			values, err := importRow(row, plan)
			if err != nil {
				return importPreviewMsg{id: id, err: fmt.Errorf("row %d: %w", i+1, err)}
			}
			preview[i] = values
		}
		insert := ""
		if inserter, ok := conn.Mutator().(db.Inserter); ok {
			insert = inserter.PreviewInsert(target, importNames(mapped), preview)
		}
		return importPreviewMsg{id: id, plan: plan, rows: preview, insert: insert}
	}
}

// matchImportColumns maps each source column to a table column. When the file
// has no header row, columns map by position instead.
func matchImportColumns(target []db.Column, source []string, width int) ([]int, []db.Column, error) {
	if len(source) == 0 {
		if width > len(target) {
			return nil, nil, fmt.Errorf("import_width:%d:%d", len(target), width)
		}
		indexes := make([]int, width)
		columns := make([]db.Column, width)
		for i := 0; i < width; i++ {
			indexes[i] = i
			columns[i] = target[i]
		}
		return indexes, columns, nil
	}
	byName := make(map[string]db.Column, len(target))
	for _, column := range target {
		byName[strings.ToLower(column.Name)] = column
	}
	var indexes []int
	var columns []db.Column
	var unknown []string
	seen := map[string]bool{}
	for i, name := range source {
		column, ok := byName[strings.ToLower(name)]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		if seen[column.Name] {
			return nil, nil, fmt.Errorf("duplicate column %q", column.Name)
		}
		seen[column.Name] = true
		indexes = append(indexes, i)
		columns = append(columns, column)
	}
	if len(unknown) > 0 {
		return nil, nil, fmt.Errorf("unknown columns: %s", strings.Join(unknown, ", "))
	}
	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("no matching columns")
	}
	return indexes, columns, nil
}

func importNames(columns []db.Column) []string {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = column.Name
	}
	return names
}

func importRow(row []any, plan importPlan) ([]any, error) {
	values := make([]any, len(plan.columns))
	for i, index := range plan.indexes {
		var raw any
		if index >= 0 && index < len(row) {
			raw = row[index]
		}
		value, err := convertImportValue(raw, plan.columns[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", plan.columns[i].Name, err)
		}
		values[i] = value
	}
	return values, nil
}

func convertImportValue(raw any, column db.Column) (any, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		if value == "" {
			return nil, nil
		}
		return parseCellValue(value, column.DataType)
	case json.Number:
		return parseCellValue(value.String(), column.DataType)
	default:
		return raw, nil
	}
}

func (m *Model) handleImportPreview(msg importPreviewMsg) tea.Cmd {
	if msg.id != m.connectSeq {
		return nil
	}
	if msg.err != nil {
		if text, ok := importErrorMessage(m.t, msg.err); ok {
			m.setStatus(text, true)
		} else {
			m.setStatus(m.t.T(i18n.KeyErrorImport, msg.err), true)
		}
		return nil
	}
	plan := msg.plan
	m.pendingImport = &plan
	preview := importPreviewText(plan, msg.rows, msg.insert)
	m.confirm.Open(fmt.Sprintf(m.t.T(i18n.KeyImportTitle), qualifiedTable(plan.table.Schema, plan.table.Name)), preview)
	return nil
}

// importErrorMessage turns the sentinel encoded by the background worker into
// a translated message. Unknown errors fall back to the generic import error.
func importErrorMessage(t *i18n.Translator, err error) (string, bool) {
	text := err.Error()
	switch {
	case text == "import_empty":
		return t.T(i18n.KeyErrorImportEmpty), true
	case strings.HasPrefix(text, "import_width:"):
		var have, want int
		if _, scanErr := fmt.Sscanf(text, "import_width:%d:%d", &have, &want); scanErr == nil {
			return t.T(i18n.KeyErrorImportWidth, have, want), true
		}
	case strings.HasPrefix(text, "unknown columns: "):
		return t.T(i18n.KeyErrorImportColumns, strings.TrimPrefix(text, "unknown columns: ")), true
	}
	return "", false
}

func importPreviewText(plan importPlan, rows [][]any, insert string) string {
	names := importNames(plan.columns)
	header := "columns: " + strings.Join(names, ", ")
	if insert == "" {
		return header
	}
	// The confirm box truncates each line, so split before VALUES to keep the
	// literal values readable.
	insert = strings.ReplaceAll(insert, " VALUES ", "\n  VALUES ")
	return header + "\n\n" + insert
}

func (m *Model) importCmd(plan importPlan) tea.Cmd {
	conn := m.conn
	id := m.connectSeq
	send := m.send
	ctx, cancel := context.WithCancel(context.Background())
	m.mutationCancel = cancel
	m.importing = true
	m.setStatus(m.t.T(i18n.KeyStatusImporting, plan.table.Name), false)
	return func() tea.Msg {
		defer cancel()
		ctx, cancelTimeout := context.WithTimeout(ctx, importTimeout)
		defer cancelTimeout()
		inserter, ok := conn.Mutator().(db.Inserter)
		if !ok {
			return importedMsg{id: id, table: plan.table, err: fmt.Errorf("insert is not supported for %s", conn.Type())}
		}
		reader, err := importer.Open(plan.path, plan.format, plan.header)
		if err != nil {
			return importedMsg{id: id, table: plan.table, err: err}
		}
		defer reader.Close()
		source := &importSource{reader: reader, plan: plan}
		var last int64
		onProgress := func(rows int64) {
			if send == nil || rows-last < importProgressStep {
				return
			}
			last = rows
			send(importProgressMsg{label: plan.table.Name, rows: rows})
		}
		rows, err := inserter.InsertRows(ctx, plan.table, importNames(plan.columns), source, onProgress)
		if err != nil {
			return importedMsg{id: id, table: plan.table, rows: rows, err: err}
		}
		return importedMsg{id: id, table: plan.table, rows: rows}
	}
}

// importSource adapts the file reader and its column mapping to db.RowSource.
type importSource struct {
	reader importer.Reader
	plan   importPlan
	values []any
	index  int
	err    error
}

func (s *importSource) Next() bool {
	if s.err != nil {
		return false
	}
	row, err := s.reader.Next()
	if errors.Is(err, io.EOF) {
		return false
	}
	if err != nil {
		s.err = err
		return false
	}
	values, err := importRow(row, s.plan)
	if err != nil {
		s.index++
		s.err = fmt.Errorf("row %d: %w", s.index, err)
		return false
	}
	s.values = values
	s.index++
	return true
}

func (s *importSource) Values() []any { return s.values }

func (s *importSource) Err() error { return s.err }
