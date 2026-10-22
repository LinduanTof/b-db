package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	DefaultConnection string       `yaml:"default_connection"`
	UI                UIConfig     `yaml:"ui"`
	Query             QueryConfig  `yaml:"query"`
	Connections       []Connection `yaml:"connections"`
}

type UIConfig struct {
	Language string `yaml:"language"`
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

func Load(path string) (*Config, []string, error) {
	cfg := Defaults()
	if path == "" {
		path = DefaultPath()
	}

	var warnings []string
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		warnings = append(warnings, fmt.Sprintf("config file %s not found; using defaults", path))
	case err != nil:
		return nil, nil, fmt.Errorf("read config %s: %w", path, err)
	default:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
			warnings = append(warnings, fmt.Sprintf("config file %s is readable by other users (mode %04o); consider chmod 600", path, info.Mode().Perm()))
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

func (c *Config) expandEnv() error {
	for i := range c.Connections {
		conn := &c.Connections[i]
		fields := []struct {
			name string
			ptr  *string
		}{
			{"name", &conn.Name},
			{"type", &conn.Type},
			{"host", &conn.Host},
			{"user", &conn.User},
			{"password", &conn.Password},
			{"database", &conn.Database},
			{"sslmode", &conn.SSLMode},
			{"path", &conn.Path},
			{"addr", &conn.Addr},
			{"uri", &conn.URI},
		}
		for _, f := range fields {
			v, err := expand(conn.Name, f.name, *f.ptr)
			if err != nil {
				return err
			}
			*f.ptr = v
		}
	}
	return nil
}

func (c *Config) Validate() error {
	seen := make(map[string]bool, len(c.Connections))
	for i, conn := range c.Connections {
		if conn.Name == "" {
			return fmt.Errorf("connections[%d]: name is required", i)
		}
		if seen[conn.Name] {
			return fmt.Errorf("connections[%d]: duplicate name %q", i, conn.Name)
		}
		seen[conn.Name] = true
		if conn.Port < 0 || conn.Port > 65535 {
			return fmt.Errorf("connection %q: invalid port %d", conn.Name, conn.Port)
		}
		switch conn.Type {
		case TypePostgres, TypeMySQL:
			if conn.Host == "" || conn.User == "" || conn.Database == "" {
				return fmt.Errorf("connection %q: host, user and database are required for %s", conn.Name, conn.Type)
			}
		case TypeSQLite:
			if conn.Path == "" {
				return fmt.Errorf("connection %q: path is required for sqlite", conn.Name)
			}
		case TypeRedis:
			if conn.Addr == "" {
				return fmt.Errorf("connection %q: addr is required for redis", conn.Name)
			}
		case TypeMongo:
			if conn.URI == "" && conn.Host == "" {
				return fmt.Errorf("connection %q: uri or host is required for mongo", conn.Name)
			}
		default:
			return fmt.Errorf("connection %q: unsupported type %q", conn.Name, conn.Type)
		}
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
	}
	return 0
}
