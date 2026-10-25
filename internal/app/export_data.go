package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/db"
	"github.com/LinduanTof/b-db/internal/export"
	"github.com/LinduanTof/b-db/internal/i18n"
)

const (
	exportTimeout = 30 * time.Minute
	exportRowStep = 100_000
)

type exportTable struct {
	schema string
	name   string
}

func sqlExportable(typ string) bool {
	switch typ {
	case config.TypePostgres, config.TypeMySQL, config.TypeSQLite:
		return true
	default:
		return false
	}
}

func (m *Model) exportTableCmd(schema, table, kind string) tea.Cmd {
	conn := m.conn
	if conn == nil {
		m.setStatus(m.t.T(i18n.KeyStatusNoConnection), true)
		return nil
	}
	if !sqlExportable(conn.Type()) {
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, conn.Type()), true)
		return nil
	}
	outDir := m.outDir
	send := m.send
	return func() tea.Msg {
		notifyExport(send, table, 0, 0)
		ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
		defer cancel()
		onRows := func(rows int64) { notifyExportRows(send, table, rows) }
		path, err := writeTableExport(ctx, conn, schema, table, kind, outDir, onRows)
		if err != nil {
			return exportedMsg{err: err}
		}
		return exportedMsg{path: path}
	}
}

func notifyExport(send func(tea.Msg), label string, done, total int) {
	if send != nil {
		send(exportProgressMsg{label: label, done: done, total: total})
	}
}

func notifyExportRows(send func(tea.Msg), label string, rows int64) {
	if send != nil {
		send(exportProgressMsg{label: label, rows: rows})
	}
}

func writeTableExport(ctx context.Context, conn db.Conn, schema, table, kind, outDir string, onRows func(int64)) (string, error) {
	var ddl string
	if kind == "sql" || kind == "ddl" {
		value, err := conn.Catalog().DDL(ctx, schema, table)
		if err != nil {
			return "", err
		}
		ddl = value
	}
	if kind == "ddl" {
		path := exportPath(outDir, table, "sql", false)
		return path, writeExportFile(path, func(f *os.File) error {
			return export.SQLStructure(f, ddl)
		})
	}
	if kind == "xlsx" {
		path := exportPath(outDir, table, "xlsx", false)
		sheets := []export.SheetSource{{
			Name: table,
			Open: func() (db.RowStream, error) {
				return openExportStream(ctx, conn, schema, table, onRows)
			},
		}}
		return path, writeExportFile(path, func(f *os.File) error {
			return export.XLSXStream(f, sheets)
		})
	}
	stream, err := openExportStream(ctx, conn, schema, table, onRows)
	if err != nil {
		return "", err
	}
	path := exportPath(outDir, table, kind, false)
	err = writeExportFile(path, func(f *os.File) error {
		switch kind {
		case "sql":
			return export.SQLTableStream(f, dialectFor(conn.Type()), schema, table, ddl, stream)
		case "csv":
			return export.CSVStream(f, stream)
		case "json":
			return export.JSONStream(f, stream)
		default:
			return fmt.Errorf("unsupported export %q", kind)
		}
	})
	if closeErr := stream.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func openExportStream(ctx context.Context, conn db.Conn, schema, table string, onRows func(int64)) (db.RowStream, error) {
	streamer, ok := conn.(db.Streamer)
	if !ok {
		return nil, fmt.Errorf("streaming not supported for %s", conn.Type())
	}
	stmt := "SELECT * FROM " + dialectFor(conn.Type()).Qualified(schema, table)
	stream, err := streamer.Stream(ctx, stmt, 0)
	if err != nil {
		return nil, err
	}
	if onRows != nil {
		stream = &progressRowStream{RowStream: stream, onRows: onRows}
	}
	return stream, nil
}

type progressRowStream struct {
	db.RowStream
	rows   int64
	onRows func(int64)
}

func (s *progressRowStream) Next() bool {
	if !s.RowStream.Next() {
		return false
	}
	s.rows++
	if s.onRows != nil && s.rows%exportRowStep == 0 {
		s.onRows(s.rows)
	}
	return true
}

func (m *Model) exportDatabaseCmd(session, kind string) tea.Cmd {
	target := m.cfg.Find(session)
	if target == nil {
		m.setStatus(m.t.T(i18n.KeyErrorExport, fmt.Sprintf("unknown session %q", session)), true)
		return nil
	}
	if !sqlExportable(target.Type) {
		m.setStatus(m.t.T(i18n.KeyErrorUnsupportedFeature, target.Type), true)
		return nil
	}
	conn := m.conn
	if session != m.connName || conn == nil {
		if saved, ok := m.sessions[session]; ok {
			conn = saved.conn
		} else {
			conn = nil
		}
	}
	database := sessionDatabase(session)
	if database == "" {
		database = target.Database
	}
	if database == "" {
		m.setStatus(m.t.T(i18n.KeyStatusNoDatabase), false)
		return nil
	}
	targetCfg := *target
	outDir := m.outDir
	typ := target.Type
	send := m.send
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
		defer cancel()
		if conn == nil {
			opened, err := db.Open(ctx, toDBConfig(&targetCfg))
			if err != nil {
				return exportedMsg{err: err}
			}
			defer opened.Close()
			conn = opened
		}
		notify := func(label string, done, total int) {
			notifyExport(send, label, done, total)
		}
		onRows := func(label string) func(int64) {
			return func(rows int64) { notifyExportRows(send, label, rows) }
		}
		path, err := writeDatabaseExport(ctx, conn, typ, database, kind, outDir, notify, onRows)
		if err != nil {
			return exportedMsg{err: err}
		}
		return exportedMsg{path: path}
	}
}

func writeDatabaseExport(ctx context.Context, conn db.Conn, typ, database, kind, outDir string, notify func(label string, done, total int), onRows func(label string) func(int64)) (string, error) {
	tables, err := listExportTables(ctx, conn, typ, database)
	if err != nil {
		return "", err
	}
	total := len(tables)
	notify(database, 0, total)
	tableLabel := func(table exportTable) string {
		return exportTableName(table)
	}
	switch kind {
	case "ddl":
		path := exportPath(outDir, database, "sql", false)
		return path, writeExportFile(path, func(f *os.File) error {
			for i, table := range tables {
				ddl, err := conn.Catalog().DDL(ctx, table.schema, table.name)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(f, "-- %s\n", table.name); err != nil {
					return err
				}
				if err := export.SQLStructure(f, ddl); err != nil {
					return err
				}
				if _, err := f.WriteString("\n"); err != nil {
					return err
				}
				notify(tableLabel(table), i+1, total)
			}
			return nil
		})
	case "sql":
		path := exportPath(outDir, database, "sql", false)
		return path, writeExportFile(path, func(f *os.File) error {
			for i, table := range tables {
				ddl, err := conn.Catalog().DDL(ctx, table.schema, table.name)
				if err != nil {
					return err
				}
				stream, err := openExportStream(ctx, conn, table.schema, table.name, onRows(tableLabel(table)))
				if err != nil {
					return err
				}
				err = export.SQLTableStream(f, dialectFor(typ), table.schema, table.name, ddl, stream)
				if closeErr := stream.Close(); err == nil {
					err = closeErr
				}
				if err != nil {
					return err
				}
				if _, err := f.WriteString("\n"); err != nil {
					return err
				}
				notify(tableLabel(table), i+1, total)
			}
			return nil
		})
	case "xlsx":
		sheets := make([]export.SheetSource, 0, total)
		for i, table := range tables {
			index := i
			sheets = append(sheets, export.SheetSource{
				Name: tableLabel(table),
				Open: func() (db.RowStream, error) {
					return openExportStream(ctx, conn, table.schema, table.name, onRows(tableLabel(table)))
				},
				Done: func() { notify(tableLabel(table), index+1, total) },
			})
		}
		path := exportPath(outDir, database, "xlsx", false)
		return path, writeExportFile(path, func(f *os.File) error {
			return export.XLSXStream(f, sheets)
		})
	case "json":
		streams := make([]export.TableStream, 0, total)
		for i, table := range tables {
			index := i
			streams = append(streams, export.TableStream{
				Schema: table.schema,
				Table:  table.name,
				Open: func() (db.RowStream, error) {
					return openExportStream(ctx, conn, table.schema, table.name, onRows(tableLabel(table)))
				},
				Done: func() { notify(tableLabel(table), index+1, total) },
			})
		}
		path := exportPath(outDir, database, "json", false)
		return path, writeExportFile(path, func(f *os.File) error {
			return export.JSONTablesStream(f, streams)
		})
	case "csv":
		dir := exportPath(outDir, database, "", true)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		for i, table := range tables {
			stream, err := openExportStream(ctx, conn, table.schema, table.name, onRows(tableLabel(table)))
			if err != nil {
				return "", err
			}
			path := filepath.Join(dir, sanitizeExportName(tableLabel(table))+".csv")
			err = writeExportFile(path, func(f *os.File) error { return export.CSVStream(f, stream) })
			if closeErr := stream.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return "", err
			}
			notify(tableLabel(table), i+1, total)
		}
		return dir, nil
	default:
		return "", fmt.Errorf("unsupported export %q", kind)
	}
}

func listExportTables(ctx context.Context, conn db.Conn, typ, database string) ([]exportTable, error) {
	catalog := conn.Catalog()
	if typ == config.TypeMySQL {
		objects, err := catalog.Tables(ctx, database)
		if err != nil {
			return nil, err
		}
		out := make([]exportTable, 0, len(objects))
		for _, obj := range objects {
			out = append(out, exportTable{schema: database, name: obj.Name})
		}
		return out, nil
	}
	schemas, err := catalog.Schemas(ctx)
	if err != nil {
		return nil, err
	}
	var out []exportTable
	for _, schema := range schemas {
		objects, err := catalog.Tables(ctx, schema)
		if err != nil {
			continue
		}
		for _, obj := range objects {
			out = append(out, exportTable{schema: schema, name: obj.Name})
		}
	}
	return out, nil
}

func exportTableName(table exportTable) string {
	if table.schema == "" || table.schema == "main" {
		return table.name
	}
	return table.schema + "." + table.name
}

func exportPath(outDir, prefix, ext string, dir bool) string {
	name := fmt.Sprintf("bdb-%s-%s", sanitizeExportName(prefix), time.Now().Format("20060102-150405.000000000"))
	if !dir && ext != "" {
		name += "." + ext
	}
	return filepath.Join(outDir, name)
}

func (m *Model) applyExportDir(value string) {
	dir := value
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorExportDir, err), true)
			return
		}
		dir = wd
	} else if !filepath.IsAbs(dir) {
		wd, err := os.Getwd()
		if err != nil {
			m.setStatus(m.t.T(i18n.KeyErrorExportDir, err), true)
			return
		}
		dir = filepath.Join(wd, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.dirInput.Open(value)
		m.dirInput.SetError(m.t.T(i18n.KeyErrorExportDir, err))
		return
	}
	m.outDir = dir
	saved := dir
	if value == "" {
		saved = ""
	}
	_ = config.SaveExportDirState(saved)
	m.setStatus(m.t.T(i18n.KeyStatusExportDir, dir), false)
}

func writeExportFile(path string, write func(*os.File) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	err = write(f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

func sanitizeExportName(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		}
		return r
	}, value)
}
