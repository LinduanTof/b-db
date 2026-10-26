package sql

import (
	"context"
	"testing"

	"github.com/LinduanTof/dbrix/internal/db"
)

func TestMemorySQLiteUsesSingleConnection(t *testing.T) {
	opened, err := (sqliteDriver{}).Open(context.Background(), db.Config{
		Name:   "mem",
		Type:   "sqlite",
		Params: map[string]string{"path": ":memory:"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	c := opened.(*conn)
	if got := c.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}
}
