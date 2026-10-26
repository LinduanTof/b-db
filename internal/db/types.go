package db

import (
	"context"
	"time"
)

type Capability uint32

const (
	CapQuery Capability = 1 << iota
	CapCatalog
	CapMutate
)

func (c Capability) Has(other Capability) bool { return c&other == other }

type Config struct {
	Name   string
	Type   string
	Params map[string]string
}

func (c Config) Get(key string) string { return c.Params[key] }

type Driver interface {
	Type() string
	Open(ctx context.Context, cfg Config) (Conn, error)
}

type Conn interface {
	Name() string
	Type() string
	Ping(ctx context.Context) error
	Close() error
	Capabilities() Capability
	Catalog() Catalog
	Querier() Querier
	Mutator() Mutator
}

type TableRef struct {
	Schema string
	Name   string
}

type Mutator interface {
	UpdateCell(ctx context.Context, table TableRef, key map[string]any, column string, value any) (int64, error)
	DeleteRow(ctx context.Context, table TableRef, key map[string]any) (int64, error)
	PreviewUpdate(table TableRef, key map[string]any, column string, value any) string
	PreviewDelete(table TableRef, key map[string]any) string
}

// Inserter is implemented by connections that can insert rows. It is separate
// from Mutator because bulk import and the row form do not need a primary key.
// InsertRows must be atomic: either every row is written or none is. It reads
// rows from a RowSource so imports never buffer the whole file, and calls
// onProgress every time a row is written when onProgress is not nil.
type Inserter interface {
	InsertRows(ctx context.Context, table TableRef, columns []string, rows RowSource, onProgress func(int64)) (int64, error)
	PreviewInsert(table TableRef, columns []string, rows [][]any) string
}

// RowSource is a forward-only stream of values for InsertRows. Values must be
// valid until the next call to Next.
type RowSource interface {
	Next() bool
	Values() []any
	Err() error
}

// SliceRows adapts an in-memory slice of rows to RowSource.
type SliceRows struct {
	rows  [][]any
	index int
}

func NewSliceRows(rows [][]any) *SliceRows { return &SliceRows{rows: rows, index: -1} }

func (s *SliceRows) Next() bool {
	s.index++
	return s.index < len(s.rows)
}

func (s *SliceRows) Values() []any { return s.rows[s.index] }

func (s *SliceRows) Err() error { return nil }

type Catalog interface {
	Schemas(ctx context.Context) ([]string, error)
	Tables(ctx context.Context, schema string) ([]Object, error)
	Columns(ctx context.Context, schema, table string) ([]Column, error)
	Indexes(ctx context.Context, schema, table string) ([]Index, error)
	ForeignKeys(ctx context.Context, schema, table string) ([]ForeignKey, error)
	DDL(ctx context.Context, schema, table string) (string, error)
}

// DatabaseLister is implemented by catalogs that can enumerate databases on
// the server, such as PostgreSQL.
type DatabaseLister interface {
	Databases(ctx context.Context) ([]string, error)
}

// Versioner is implemented by connections that can report a server version.
type Versioner interface {
	Version(ctx context.Context) (string, error)
}

// TableStat holds approximate per-table metadata. Negative values mean the
// driver could not determine them.
type TableStat struct {
	Name    string
	Rows    int64
	Size    int64
	Columns int64
	Comment string
}

// TableStatProvider is implemented by connections that can report table
// statistics for a schema.
type TableStatProvider interface {
	TableStats(ctx context.Context, schema string) ([]TableStat, error)
}

// Querier executes statements and buffers the whole result.
type Querier interface {
	Query(ctx context.Context, stmt string, maxRows int) (*ResultSet, error)
	Exec(ctx context.Context, stmt string) (*ExecResult, error)
}

// RowStream is a forward-only cursor over query results. Callers must Close
// it. It lets large results be exported without buffering every row.
type RowStream interface {
	Columns() []ColumnMeta
	Next() bool
	Values() ([]any, error)
	Err() error
	Close() error
}

// Streamer is implemented by connections that can stream rows.
type Streamer interface {
	Stream(ctx context.Context, stmt string, maxRows int) (RowStream, error)
}

type Object struct {
	Schema string
	Name   string
	Kind   string
}

type ColumnMeta struct {
	Name         string
	DatabaseType string
	Nullable     bool
}

type ResultSet struct {
	Columns   []ColumnMeta
	Rows      [][]any
	Truncated bool
	Elapsed   time.Duration
}

type ExecResult struct {
	Affected int64
	Elapsed  time.Duration
}

type Column struct {
	Name       string
	DataType   string
	Nullable   bool
	Default    string
	PrimaryKey bool
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

type ForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
}
