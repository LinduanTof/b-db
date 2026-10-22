package db

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Driver{}
)

func Register(d Driver) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[d.Type()] = d
}

func Open(ctx context.Context, cfg Config) (Conn, error) {
	registryMu.RLock()
	d, ok := registry[cfg.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported database type %q", cfg.Type)
	}
	return d.Open(ctx, cfg)
}

func Supported() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for typ := range registry {
		out = append(out, typ)
	}
	sort.Strings(out)
	return out
}
