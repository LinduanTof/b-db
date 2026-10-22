package dialect

import (
	"strconv"
	"strings"
)

type Dialect interface {
	Quote(ident string) string
	Placeholder(n int) string
	Qualified(schema, name string) string
}

type Postgres struct{}

func (Postgres) Quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (Postgres) Placeholder(n int) string { return "$" + strconv.Itoa(n) }

func (d Postgres) Qualified(schema, name string) string {
	if schema == "" {
		return d.Quote(name)
	}
	return d.Quote(schema) + "." + d.Quote(name)
}

type SQLite struct{}

func (SQLite) Quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (SQLite) Placeholder(int) string { return "?" }

func (d SQLite) Qualified(schema, name string) string {
	if schema == "" || schema == "main" {
		return d.Quote(name)
	}
	return d.Quote(schema) + "." + d.Quote(name)
}

type MySQL struct{}

func (MySQL) Quote(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

func (MySQL) Placeholder(int) string { return "?" }

func (d MySQL) Qualified(schema, name string) string {
	if schema == "" {
		return d.Quote(name)
	}
	return d.Quote(schema) + "." + d.Quote(name)
}
