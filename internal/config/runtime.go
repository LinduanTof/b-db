package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type runtimeFile struct {
	Connections []Connection `yaml:"connections"`
}

func RuntimeConnectionsPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "dbrix", "connections.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "dbrix", "connections.yaml")
	}
	return filepath.Join(home, ".local", "state", "dbrix", "connections.yaml")
}

func loadRuntimeRaw() ([]Connection, error) {
	data, err := os.ReadFile(RuntimeConnectionsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var file runtimeFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Connections, nil
}

func LoadRuntimeConnections() ([]Connection, error) {
	connections, err := loadRuntimeRaw()
	if err != nil {
		return nil, err
	}
	for i := range connections {
		if err := connections[i].expandEnv(); err != nil {
			return nil, err
		}
	}
	return connections, nil
}

func AppendRuntimeConnection(conn Connection) error {
	connections, err := loadRuntimeRaw()
	if err != nil {
		return err
	}
	replaced := false
	for i := range connections {
		if connections[i].Name == conn.Name {
			connections[i] = conn
			replaced = true
			break
		}
	}
	if !replaced {
		connections = append(connections, conn)
	}
	return saveRuntime(connections)
}

func MergeRuntime(cfg *Config) error {
	connections, err := LoadRuntimeConnections()
	if err != nil {
		return err
	}
	for _, conn := range connections {
		if cfg.Find(conn.Name) != nil {
			continue
		}
		if err := conn.validate(); err != nil {
			continue
		}
		cfg.Connections = append(cfg.Connections, conn)
	}
	return nil
}

func saveRuntime(connections []Connection) error {
	path := RuntimeConnectionsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(runtimeFile{Connections: connections})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".connections-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
