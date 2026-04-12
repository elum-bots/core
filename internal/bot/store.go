package bot

import (
	"context"
	"sync"
	"time"
)

type StateStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string][]byte)}
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.items[key]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *MemoryStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := make([]byte, len(value))
	copy(v, value)
	m.items[key] = v
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}
