package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddDedupeAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	s := Load(path, 3)
	if len(s.Entries()) != 0 {
		t.Fatalf("expected empty history")
	}
	s.Add("select 1")
	s.Add("select 1")
	s.Add("select 2")
	s.Add("select 3")
	s.Add("select 4")
	got := s.Entries()
	if len(got) != 3 {
		t.Fatalf("entries = %v", got)
	}
	if got[0] != "select 2" || got[2] != "select 4" {
		t.Fatalf("entries = %v", got)
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "bdb", "history")
	s := Load(path, 100)
	s.Add("select '中文'")
	s.Add("select 2;\nselect 3;")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}

	again := Load(path, 100)
	entries := again.Entries()
	if len(entries) != 2 || entries[0] != "select '中文'" || entries[1] != "select 2;\nselect 3;" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestLoadSkipsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte("not json\n\"select 1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := Load(path, 10).Entries()
	if len(entries) != 1 || entries[0] != "select 1" {
		t.Fatalf("entries = %#v", entries)
	}
}
