package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/LinduanTof/b-db/internal/db"
)

func init() {
	db.Register(driver{})
}

type driver struct{}

func (driver) Type() string { return "mongo" }

func (driver) Open(_ context.Context, cfg db.Config) (db.Conn, error) {
	uri := cfg.Get("uri")
	if uri == "" {
		host := cfg.Get("host")
		if host == "" {
			host = "127.0.0.1"
		}
		port := 27017
		if value := cfg.Get("port"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid mongo port %q: %w", value, err)
			}
			port = parsed
		}
		auth := ""
		if user := cfg.Get("user"); user != "" {
			auth = url.UserPassword(user, cfg.Get("password")).String() + "@"
		}
		uri = fmt.Sprintf("mongodb://%s%s/%s", auth, net.JoinHostPort(host, strconv.Itoa(port)), cfg.Get("database"))
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	return &conn{name: cfg.Name, client: client}, nil
}

type conn struct {
	name   string
	client *mongo.Client
}

func (c *conn) Name() string { return c.name }

func (c *conn) Type() string { return "mongo" }

func (c *conn) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, readpref.Primary())
}

func (c *conn) Close() error { return c.client.Disconnect(context.Background()) }

func (c *conn) Capabilities() db.Capability { return db.CapQuery | db.CapCatalog }

func (c *conn) Catalog() db.Catalog { return catalog{c: c} }

func (c *conn) Querier() db.Querier { return querier{c: c} }

func (c *conn) Mutator() db.Mutator { return nil }

type catalog struct{ c *conn }

func (c catalog) Schemas(ctx context.Context) ([]string, error) {
	names, err := c.c.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		switch name {
		case "admin", "local", "config":
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (c catalog) Tables(ctx context.Context, schema string) ([]db.Object, error) {
	names, err := c.c.client.Database(schema).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]db.Object, 0, len(names))
	for _, name := range names {
		out = append(out, db.Object{Schema: schema, Name: name, Kind: "collection"})
	}
	return out, nil
}

func (c catalog) Columns(ctx context.Context, schema, table string) ([]db.Column, error) {
	var doc bson.M
	err := c.c.client.Database(schema).Collection(table).FindOne(ctx, bson.M{}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]db.Column, 0, len(keys))
	for _, key := range keys {
		out = append(out, db.Column{
			Name:     key,
			DataType: fmt.Sprintf("%T", doc[key]),
			Nullable: true,
		})
	}
	return out, nil
}

func (c catalog) Indexes(ctx context.Context, schema, table string) ([]db.Index, error) {
	cursor, err := c.c.client.Database(schema).Collection(table).Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	out := make([]db.Index, 0, len(raw))
	for _, item := range raw {
		index := db.Index{}
		if name, ok := item["name"].(string); ok {
			index.Name = name
		}
		if unique, ok := item["unique"].(bool); ok {
			index.Unique = unique
		}
		index.Columns = indexColumns(item["key"])
		out = append(out, index)
	}
	return out, nil
}

func indexColumns(value any) []string {
	switch v := value.(type) {
	case bson.D:
		out := make([]string, 0, len(v))
		for _, element := range v {
			out = append(out, element.Key)
		}
		return out
	case bson.M:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	default:
		return nil
	}
}

func (c catalog) ForeignKeys(context.Context, string, string) ([]db.ForeignKey, error) {
	return nil, nil
}

func (c catalog) DDL(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("mongo collections do not have DDL")
}

type querier struct{ c *conn }

func (q querier) Query(ctx context.Context, stmt string, maxRows int) (*db.ResultSet, error) {
	request, err := parseCommand(stmt)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	collection := q.c.client.Database(request.database).Collection(request.collection)

	var result *db.ResultSet
	switch request.op {
	case "count":
		count, err := collection.CountDocuments(ctx, request.filter)
		if err != nil {
			return nil, err
		}
		result = &db.ResultSet{
			Columns: []db.ColumnMeta{{Name: "count"}},
			Rows:    [][]any{{count}},
		}
	case "aggregate":
		cursor, err := collection.Aggregate(ctx, request.pipeline)
		if err != nil {
			return nil, err
		}
		result, err = documentsResult(ctx, cursor, maxRows)
		if err != nil {
			return nil, err
		}
	case "findone":
		var doc bson.M
		err := collection.FindOne(ctx, request.filter).Decode(&doc)
		if errors.Is(err, mongo.ErrNoDocuments) {
			result = &db.ResultSet{}
		} else if err != nil {
			return nil, err
		} else {
			result = documentsToResult([]bson.M{doc}, maxRows)
		}
	default:
		limit := int64(0)
		if maxRows > 0 {
			limit = int64(maxRows) + 1
		}
		cursor, err := collection.Find(ctx, request.filter, options.Find().SetLimit(limit))
		if err != nil {
			return nil, err
		}
		result, err = documentsResult(ctx, cursor, maxRows)
		if err != nil {
			return nil, err
		}
	}
	result.Elapsed = time.Since(start)
	return result, nil
}

func (q querier) Exec(context.Context, string) (*db.ExecResult, error) {
	return nil, fmt.Errorf("mongo connections are read-only")
}

type command struct {
	op         string
	database   string
	collection string
	filter     bson.M
	pipeline   []bson.M
}

func parseCommand(stmt string) (command, error) {
	trimmed := strings.TrimSpace(stmt)
	if trimmed == "" {
		return command{}, fmt.Errorf("empty mongo command")
	}

	op := "find"
	rest := trimmed
	first := strings.Fields(trimmed)[0]
	switch strings.ToLower(first) {
	case "find", "findone", "aggregate", "count":
		op = strings.ToLower(first)
		rest = strings.TrimSpace(trimmed[len(first):])
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return command{}, fmt.Errorf("usage: find <database>.<collection> [json]")
	}
	target := fields[0]
	parts := strings.SplitN(target, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return command{}, fmt.Errorf("target must be <database>.<collection>")
	}

	jsonPart := strings.TrimSpace(rest[len(target):])
	req := command{op: op, database: parts[0], collection: parts[1]}
	switch op {
	case "aggregate":
		req.pipeline = []bson.M{}
		if jsonPart != "" {
			if err := bson.UnmarshalExtJSON([]byte(jsonPart), false, &req.pipeline); err != nil {
				return command{}, fmt.Errorf("invalid pipeline JSON: %w", err)
			}
		}
	default:
		req.filter = bson.M{}
		if jsonPart != "" {
			if err := bson.UnmarshalExtJSON([]byte(jsonPart), false, &req.filter); err != nil {
				return command{}, fmt.Errorf("invalid filter JSON: %w", err)
			}
		}
	}
	return req, nil
}

func documentsResult(ctx context.Context, cursor *mongo.Cursor, maxRows int) (*db.ResultSet, error) {
	defer cursor.Close(ctx)
	docs := make([]bson.M, 0, 32)
	truncated := false
	for cursor.Next(ctx) {
		if maxRows > 0 && len(docs) >= maxRows {
			truncated = true
			break
		}
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	result := documentsToResult(docs, maxRows)
	result.Truncated = truncated
	return result, nil
}

func documentsToResult(docs []bson.M, maxRows int) *db.ResultSet {
	seen := map[string]bool{}
	columns := make([]string, 0)
	for _, doc := range docs {
		for key := range doc {
			if !seen[key] {
				seen[key] = true
				columns = append(columns, key)
			}
		}
	}
	sort.Strings(columns)

	rows := make([][]any, 0, len(docs))
	for _, doc := range docs {
		if maxRows > 0 && len(rows) >= maxRows {
			break
		}
		row := make([]any, len(columns))
		for i, column := range columns {
			row[i] = formatMongoValue(doc[column])
		}
		rows = append(rows, row)
	}

	cols := make([]db.ColumnMeta, len(columns))
	for i, column := range columns {
		cols[i] = db.ColumnMeta{Name: column, Nullable: true}
	}
	return &db.ResultSet{Columns: cols, Rows: rows}
}

func formatMongoValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string, bool, int64, float64:
		return v
	case int32:
		return int64(v)
	case bson.ObjectID:
		return v.Hex()
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
		return fmt.Sprint(v)
	}
}
