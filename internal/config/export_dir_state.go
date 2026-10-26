package config

import (
	"os"
	"path/filepath"
	"strings"
)

func ExportDirStatePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "dbrix", "export_dir")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "dbrix", "export_dir")
	}
	return filepath.Join(home, ".local", "state", "dbrix", "export_dir")
}

// LoadExportDirState reports the saved export directory. The boolean is false
// when no state file exists; an empty value with true means the user cleared
// it and the working directory should be used.
func LoadExportDirState() (string, bool) {
	data, err := os.ReadFile(ExportDirStatePath())
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

func SaveExportDirState(dir string) error {
	path := ExportDirStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(dir)+"\n"), 0o600)
}
