package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DefaultLimit = 500

type Store struct {
	mu      sync.Mutex
	path    string
	limit   int
	entries []string
}

func DefaultPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "bdb", "history")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "bdb", "history")
	}
	return filepath.Join(home, ".local", "state", "bdb", "history")
}

func Load(path string, limit int) *Store {
	if limit <= 0 {
		limit = DefaultLimit
	}
	s := &Store{path: path, limit: limit}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var stmt string
		if err := json.Unmarshal(scanner.Bytes(), &stmt); err != nil {
			continue
		}
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			s.entries = append(s.entries, stmt)
		}
	}
	if len(s.entries) > s.limit {
		s.entries = s.entries[len(s.entries)-s.limit:]
	}
	// A malformed tail is ignored: history is best-effort.
	_ = scanner.Err()
	return s
}

func (s *Store) Entries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *Store) Add(stmt string) bool {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) > 0 && s.entries[len(s.entries)-1] == stmt {
		return false
	}
	s.entries = append(s.entries, stmt)
	if len(s.entries) > s.limit {
		s.entries = s.entries[len(s.entries)-s.limit:]
	}
	return true
}

func (s *Store) Save() error {
	s.mu.Lock()
	entries := make([]string, len(s.entries))
	copy(entries, s.entries)
	path := s.path
	s.mu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".history-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	w := bufio.NewWriter(tmp)
	enc := json.NewEncoder(w)
	for _, stmt := range entries {
		if err := enc.Encode(stmt); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
