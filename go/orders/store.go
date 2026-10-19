package orders

import (
	"context"
	"sync"
)

// Store is the key-value interface the repository depends on. Get reports
// found=false for a missing key. Errors come from the driver.
type Store interface {
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	Put(ctx context.Context, key string, value []byte) error
}

// MemoryStore is the default in-process Store.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string][]byte
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string][]byte)}
}

func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, found := s.records[key]
	return value, found, nil
}

func (s *MemoryStore) Put(_ context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[key] = value
	return nil
}

// Len reports how many records the store holds.
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
