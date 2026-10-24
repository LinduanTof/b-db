package config

import (
	"os"
	"path/filepath"
	"strings"
)

func LayoutStatePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "bdb", "layout")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "bdb", "layout")
	}
	return filepath.Join(home, ".local", "state", "bdb", "layout")
}

func LoadLayoutState() string {
	data, err := os.ReadFile(LayoutStatePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SaveLayoutState(name string) error {
	path := LayoutStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(NormalizeLayout(name)+"\n"), 0o600)
}
