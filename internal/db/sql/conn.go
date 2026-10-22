package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/linduan-guan/b-db/internal/db"
	"github.com/linduan-guan/b-db/internal/db/dialect"
)

func init() {
	db.Register(pgDriver{})
	db.Register(mysqlDriver{})
	db.Register(sqliteDriver{})
}

type pgDriver struct{}

func (pgDriver) Type() string { return "postgres" }

func (pgDriver) Open(_ context.Context, cfg db.Config) (db.Conn, error) {
	port := 5432
	if p := cfg.Get("port"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid postgres port %q: %w", p, err)
		}
		port = n
	}
	sslMode := cfg.Get("sslmode")
	if sslMode == "" {
		sslMode = "prefer"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Get("user"), cfg.Get("password")),
		Host:   net.JoinHostPort(cfg.Get("host"), strconv.Itoa(port)),
		Path:   "/" + cfg.Get("database"),
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()

	sqlDB, err := stdsql.Open("pgx", u.String())
	if err != nil {
		return nil, err
	}
	return newConn(cfg.Name, "postgres", sqlDB, dialect.Postgres{}), nil
}

type mysqlDriver struct{}

func (mysqlDriver) Type() string { return "mysql" }

func (mysqlDriver) Open(_ context.Context, cfg db.Config) (db.Conn, error) {
	port := 3306
	if p := cfg.Get("port"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid mysql port %q: %w", p, err)
		}
		port = n
	}
	c := mysqldriver.NewConfig()
	c.User = cfg.Get("user")
	c.Passwd = cfg.Get("password")
	c.Net = "tcp"
	c.Addr = net.JoinHostPort(cfg.Get("host"), strconv.Itoa(port))
	c.DBName = cfg.Get("database")
	c.ParseTime = true
	c.Timeout = 10 * time.Second
	c.ReadTimeout = 30 * time.Second
	c.WriteTimeout = 30 * time.Second
	c.MultiStatements = false
	c.Collation = "utf8mb4_general_ci"
	c.Params = map[string]string{"charset": "utf8mb4"}

	sqlDB, err := stdsql.Open("mysql", c.FormatDSN())
	if err != nil {
		return nil, err
	}
	return newConn(cfg.Name, "mysql", sqlDB, dialect.MySQL{}), nil
}

type sqliteDriver struct{}

func (sqliteDriver) Type() string { return "sqlite" }

func (sqliteDriver) Open(_ context.Context, cfg db.Config) (db.Conn, error) {
	path := cfg.Get("path")
	if path == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}
	sqlDB, err := stdsql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return newConn(cfg.Name, "sqlite", sqlDB, dialect.SQLite{}), nil
}

type conn struct {
	name string
	typ  string
	db   *stdsql.DB
	d    dialect.Dialect
}

func newConn(name, typ string, sqlDB *stdsql.DB, d dialect.Dialect) *conn {
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return &conn{name: name, typ: typ, db: sqlDB, d: d}
}

func (c *conn) Name() string { return c.name }

func (c *conn) Type() string { return c.typ }

func (c *conn) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }

func (c *conn) Close() error { return c.db.Close() }

func (c *conn) Capabilities() db.Capability { return db.CapQuery | db.CapCatalog }

func (c *conn) Catalog() db.Catalog { return catalog{c: c} }

func (c *conn) Querier() db.Querier { return querier{c: c} }

func (c *conn) Mutator() db.Mutator { return mutator{c: c} }
