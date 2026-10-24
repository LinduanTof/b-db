package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	TypePostgres = "postgres"
	TypeMySQL    = "mysql"
	TypeSQLite   = "sqlite"
	TypeRedis    = "redis"
	TypeMongo    = "mongo"
)

const (
	FocusModeManual = "manual"
	FocusModeAuto   = "auto"
)

const (
	defaultTimeout = 30 * time.Second
	defaultMaxRows = 1000
)

type Duration time.Duration

func (d Duration) Std() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

type Config struct {
	// Deprecated: kept so existing config files still load. bdb no longer
	// connects automatically on startup.
	DefaultConnection string       `yaml:"default_connection"`
	UI                UIConfig     `yaml:"ui"`
	Query             QueryConfig  `yaml:"query"`
	Connections       []Connection `yaml:"connections"`

	Path string `yaml:"-"`
}

type UIConfig struct {
	Language  string `yaml:"language"`
	Theme     string `yaml:"theme"`
	FocusMode string `yaml:"focus_mode"`
}

type QueryConfig struct {
	Timeout Duration `yaml:"timeout"`
	MaxRows int      `yaml:"max_rows"`
}

type Connection struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
	Path     string `yaml:"path"`
	Addr     string `yaml:"addr"`
	DB       int    `yaml:"db"`
	URI      string `yaml:"uri"`
	ReadOnly bool   `yaml:"read_only"`
}

func Defaults() Config {
	return Config{
		UI:    UIConfig{Language: "auto"},
		Query: QueryConfig{Timeout: Duration(defaultTimeout), MaxRows: defaultMaxRows},
	}
}

func DefaultPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "bdb", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "bdb", "config.yaml")
	}
	return filepath.Join(home, ".config", "bdb", "config.yaml")
}

const defaultConfigTemplate = `# bdb configuration
# Add connections here, or press "a" inside bdb to add one.

ui:
  language: auto
  theme: default
  focus_mode: manual

query:
  timeout: 30s
  max_rows: 1000   # table browsing page size; editor queries are unlimited

connections: []

# Examples:
# - name: local-sqlite
#   type: sqlite
#   path: ./data.db
# - name: local-pg
#   type: postgres
#   host: 127.0.0.1
#   port: 5432
#   user: postgres
#   password: ${PG_PASSWORD}
#   database: postgres
#   sslmode: disable
# - name: cache
#   type: redis
#   addr: 127.0.0.1:6379
# - name: docs
#   type: mongo
#   uri: mongodb://127.0.0.1:27017
`

func ResolvePath(path string) string {
	if path != "" {
		return path
	}
	return DefaultPath()
}

func EnsureFile(path string) (bool, error) {
	path = ResolvePath(path)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	if _, err := file.WriteString(defaultConfigTemplate); err != nil {
		return false, err
	}
	return true, nil
}

func Load(path string) (*Config, []string, error) {
	cfg := Defaults()
	path = ResolvePath(path)
	cfg.Path = path

	var warnings []string
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		warnings = append(warnings, fmt.Sprintf("config file %s not found; using defaults", path))
	case err != nil:
		return nil, nil, fmt.Errorf("read config %s: %w", path, err)
	default:
		if len(bytes.TrimSpace(data)) > 0 {
			dec := yaml.NewDecoder(bytes.NewReader(data))
			dec.KnownFields(true)
			if err := dec.Decode(&cfg); err != nil {
				return nil, nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		}
		if runtime.GOOS != "windows" {
			if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
				warnings = append(warnings, fmt.Sprintf("config file %s is readable by other users (mode %04o); consider chmod 600", path, info.Mode().Perm()))
			}
		}
	}

	cfg.applyDefaults()
	if err := cfg.expandEnv(); err != nil {
		return nil, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	return &cfg, warnings, nil
}

func (c *Config) applyDefaults() {
	if c.UI.Language == "" {
		c.UI.Language = "auto"
	}
	if c.UI.Theme == "" {
		c.UI.Theme = "default"
	}
	switch c.UI.FocusMode {
	case FocusModeManual, FocusModeAuto:
	default:
		c.UI.FocusMode = FocusModeManual
	}
	if c.Query.Timeout <= 0 {
		c.Query.Timeout = Duration(defaultTimeout)
	}
	if c.Query.MaxRows <= 0 {
		c.Query.MaxRows = defaultMaxRows
	}
}

var envRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expand(connName, field, value string) (string, error) {
	if !strings.Contains(value, "${") {
		return value, nil
	}
	var missing []string
	out := envRe.ReplaceAllStringFunc(value, func(m string) string {
		name := m[2 : len(m)-1]
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return m
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("connection %q field %q: environment variable(s) not set: %s", connName, field, strings.Join(missing, ", "))
	}
	return out, nil
}

func (c *Connection) expandEnv() error {
	fields := []struct {
		name string
		ptr  *string
	}{
		{"name", &c.Name},
		{"type", &c.Type},
		{"host", &c.Host},
		{"user", &c.User},
		{"password", &c.Password},
		{"database", &c.Database},
		{"sslmode", &c.SSLMode},
		{"path", &c.Path},
		{"addr", &c.Addr},
		{"uri", &c.URI},
	}
	for _, f := range fields {
		v, err := expand(c.Name, f.name, *f.ptr)
		if err != nil {
			return err
		}
		*f.ptr = v
	}
	return nil
}

func (c *Config) expandEnv() error {
	for i := range c.Connections {
		if err := c.Connections[i].expandEnv(); err != nil {
			return err
		}
	}
	return nil
}

// ExpandConnection returns a copy of conn with ${VAR} references resolved.
// The original value is left untouched so it can still be persisted without
// writing secrets to disk.
func ExpandConnection(conn Connection) (Connection, error) {
	if err := conn.expandEnv(); err != nil {
		return Connection{}, err
	}
	return conn, nil
}

func (c Connection) validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	switch c.Type {
	case TypePostgres:
		if c.Host == "" || c.User == "" {
			return fmt.Errorf("host and user are required for postgres")
		}
	case TypeMySQL:
		if c.Host == "" || c.User == "" {
			return fmt.Errorf("host and user are required for mysql")
		}
	case TypeSQLite:
		if c.Path == "" {
			return fmt.Errorf("path is required for sqlite")
		}
	case TypeRedis:
		if c.Addr == "" {
			return fmt.Errorf("addr is required for redis")
		}
	case TypeMongo:
		if c.URI == "" && c.Host == "" {
			return fmt.Errorf("uri or host is required for mongo")
		}
	default:
		return fmt.Errorf("unsupported type %q", c.Type)
	}
	return nil
}

func (c *Config) Validate() error {
	seen := make(map[string]bool, len(c.Connections))
	for i, conn := range c.Connections {
		if err := conn.validate(); err != nil {
			if conn.Name == "" {
				return fmt.Errorf("connections[%d]: %w", i, err)
			}
			return fmt.Errorf("connection %q: %w", conn.Name, err)
		}
		if seen[conn.Name] {
			return fmt.Errorf("connection %q: duplicate name", conn.Name)
		}
		seen[conn.Name] = true
	}
	if c.DefaultConnection != "" && !seen[c.DefaultConnection] {
		return fmt.Errorf("default_connection %q does not match any connection", c.DefaultConnection)
	}
	return nil
}

func (c *Config) Find(name string) *Connection {
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			return &c.Connections[i]
		}
	}
	return nil
}

func (c Connection) EffectivePort() int {
	if c.Port != 0 {
		return c.Port
	}
	switch c.Type {
	case TypePostgres:
		return 5432
	case TypeMySQL:
		return 3306
	case TypeMongo:
		return 27017
	case TypeRedis:
		return 6379
	}
	return 0
}
