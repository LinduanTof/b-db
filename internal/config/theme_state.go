package config

import (
	"os"
	"path/filepath"
	"strings"
)

func ThemeStatePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "bdb", "theme")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "bdb", "theme")
	}
	return filepath.Join(home, ".local", "state", "bdb", "theme")
}

func LoadThemeState() string {
	data, err := os.ReadFile(ThemeStatePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SaveThemeState(name string) error {
	path := ThemeStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}
