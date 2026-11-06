package store

import (
	"sync"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type MemoryStore struct {
	mu        sync.RWMutex
	snapshots []monitor.Snapshot
	capacity  int
}

func NewMemoryStore(capacity int) *MemoryStore {
	if capacity < 1 {
		capacity = 1
	}

	return &MemoryStore{
		snapshots: make([]monitor.Snapshot, 0, capacity),
		capacity:  capacity,
	}
}

func (s *MemoryStore) AddSnapshot(snapshot monitor.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshots) >= s.capacity {
		s.snapshots = s.snapshots[1:]
	}

	s.snapshots = append(s.snapshots, snapshot)
}

func (s *MemoryStore) GetSnapshots() []monitor.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]monitor.Snapshot, len(s.snapshots))
	copy(snapshots, s.snapshots)

	return snapshots
}

// GetAll returns individual probe results across all snapshots.
func (s *MemoryStore) GetAll() []*monitor.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*monitor.ProbeResult

	for _, snapshot := range s.snapshots {
		for i := range snapshot.TCP {
			result := snapshot.TCP[i]
			results = append(results, &result)
		}

		for i := range snapshot.DNS {
			result := snapshot.DNS[i]
			results = append(results, &result)
		}
	}

	return results
}

func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.snapshots)
}
