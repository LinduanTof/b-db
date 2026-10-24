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

type Querier interface {
	Query(ctx context.Context, stmt string, maxRows int) (*ResultSet, error)
	Exec(ctx context.Context, stmt string) (*ExecResult, error)
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
