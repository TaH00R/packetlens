package store

import (
	"sync"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type MemoryStore struct {
	mu       sync.RWMutex
	results  []monitor.ProbeResult
	capacity int
}

func NewMemoryStore(capacity int) *MemoryStore {
	return &MemoryStore{
		results:  make([]monitor.ProbeResult, 0, capacity),
		capacity: capacity,
	}
}

func (s *MemoryStore) Add(result monitor.ProbeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.results) >= s.capacity {
		s.results = s.results[1:]
	}

	s.results = append(s.results, result)
}

func (s *MemoryStore) GetAll() []monitor.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]monitor.ProbeResult, len(s.results))
	copy(results, s.results)

	return results
}

func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.results)
}
