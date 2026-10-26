package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/LinduanTof/dbrix/internal/db"
)

func init() {
	db.Register(driver{})
}

type driver struct{}

func (driver) Type() string { return "redis" }

func (driver) Open(_ context.Context, cfg db.Config) (db.Conn, error) {
	addr := cfg.Get("addr")
	if addr == "" {
		return nil, fmt.Errorf("redis addr is empty")
	}
	database := 0
	if value := cfg.Get("db"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid redis db %q: %w", value, err)
		}
		database = parsed
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: cfg.Get("password"),
		DB:       database,
	})
	return &conn{name: cfg.Name, client: client, database: database}, nil
}

type conn struct {
	name     string
	client   *goredis.Client
	database int
}

func (c *conn) Name() string { return c.name }

func (c *conn) Type() string { return "redis" }

func (c *conn) Ping(ctx context.Context) error { return c.client.Ping(ctx).Err() }

func (c *conn) Close() error { return c.client.Close() }

func (c *conn) Capabilities() db.Capability { return db.CapQuery | db.CapCatalog }

func (c *conn) Catalog() db.Catalog { return catalog{c: c} }

func (c *conn) Querier() db.Querier { return querier{c: c} }

func (c *conn) Mutator() db.Mutator { return nil }

func (c *conn) Version(ctx context.Context) (string, error) {
	info, err := c.client.Info(ctx, "server").Result()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(info, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "redis_version:"); ok {
			return value, nil
		}
	}
	return "", fmt.Errorf("redis_version not found")
}

type catalog struct{ c *conn }

func (c catalog) Schemas(context.Context) ([]string, error) {
	return []string{"db" + strconv.Itoa(c.c.database)}, nil
}

func (c catalog) Tables(ctx context.Context, _ string) ([]db.Object, error) {
	var keys []string
	iterator := c.c.client.Scan(ctx, 0, "*", 500).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
		if len(keys) >= 500 {
			break
		}
	}
	if err := iterator.Err(); err != nil {
		return nil, err
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return nil, nil
	}
	pipe := c.c.client.Pipeline()
	typeCommands := make([]*goredis.StatusCmd, len(keys))
	for i, key := range keys {
		typeCommands[i] = pipe.Type(ctx, key)
	}
	_, _ = pipe.Exec(ctx)

	objects := make([]db.Object, len(keys))
	for i, key := range keys {
		kind := "string"
		if typeCommands[i] != nil {
			if value, err := typeCommands[i].Result(); err == nil {
				kind = value
			}
		}
		objects[i] = db.Object{Schema: "db" + strconv.Itoa(c.c.database), Name: key, Kind: kind}
	}
	return objects, nil
}

func (c catalog) Columns(context.Context, string, string) ([]db.Column, error) {
	return nil, nil
}

func (c catalog) Indexes(context.Context, string, string) ([]db.Index, error) {
	return nil, nil
}

func (c catalog) ForeignKeys(context.Context, string, string) ([]db.ForeignKey, error) {
	return nil, nil
}

func (c catalog) DDL(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("redis keys do not have DDL")
}

type querier struct{ c *conn }

func (q querier) Query(ctx context.Context, stmt string, maxRows int) (*db.ResultSet, error) {
	args, err := splitCommand(stmt)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty redis command")
	}

	start := time.Now()
	command := goredis.NewCmd(ctx, toArgs(args)...)
	if err := q.c.client.Process(ctx, command); err != nil {
		if errors.Is(err, goredis.Nil) {
			return &db.ResultSet{Elapsed: time.Since(start)}, nil
		}
		return nil, err
	}
	result := convert(args[0], command.Val(), maxRows)
	result.Elapsed = time.Since(start)
	return result, nil
}

func (q querier) Exec(ctx context.Context, stmt string) (*db.ExecResult, error) {
	start := time.Now()
	args, err := splitCommand(stmt)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty redis command")
	}
	if err := q.c.client.Process(ctx, goredis.NewCmd(ctx, toArgs(args)...)); err != nil {
		return nil, err
	}
	return &db.ExecResult{Elapsed: time.Since(start)}, nil
}
