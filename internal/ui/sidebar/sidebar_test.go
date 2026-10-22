package sidebar

import (
	"testing"

	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/i18n"
)

func TestHandleClickConnects(t *testing.T) {
	m := New([]config.Connection{
		{Name: "local", Type: config.TypeSQLite, Path: "x.db"},
	}, i18n.New("en"))
	m.SetSize(30, 10)

	cmd := m.HandleClick(0)
	if cmd == nil {
		t.Fatal("click on a connection should activate it")
	}
	msg, ok := cmd().(ConnectMsg)
	if !ok || msg.Name != "local" {
		t.Fatalf("message = %#v", cmd())
	}
}

func TestHandleClickOutOfRange(t *testing.T) {
	m := New(nil, i18n.New("en"))
	m.SetSize(30, 10)
	if cmd := m.HandleClick(5); cmd != nil {
		t.Fatal("click outside the list should do nothing")
	}
	if cmd := m.HandleClick(-1); cmd != nil {
		t.Fatal("negative row should do nothing")
	}
}
