package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type MemoryStore struct {
	mu             sync.RWMutex
	snapshots      []monitor.Snapshot
	capacity       int
	closed         bool
	anomalyStates  map[string]anomaly.State
	anomalyEvents  map[string]anomaly.Event
	incidentStates map[string]incident.State
	incidents      map[string]incident.Event
}

var _ Store = (*MemoryStore)(nil)

func (s *MemoryStore) SaveSnapshot(ctx context.Context, snapshot monitor.Snapshot) (monitor.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	snapshot, err := prepareSnapshot(snapshot)
	if err != nil {
		return snapshot, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return snapshot, fmt.Errorf("store is closed")
	}
	for _, existing := range s.snapshots {
		if existing.ID == snapshot.ID {
			return snapshot, fmt.Errorf("duplicate snapshot ID")
		}
	}
	ordinal := 0
	for _, group := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
		for _, p := range group {
			key := p.Type + ":" + p.Target
			next, event := anomaly.Process(s.anomalyStates[key], anomaly.DefaultConfig(), snapshot.ID, ordinal, snapshot.Timestamp, p)
			s.anomalyStates[key] = next
			if event != nil {
				s.anomalyEvents[event.ID] = *event
			}
			iState, iEvent := incident.Process(s.incidentStates[key], incident.DefaultConfig(), snapshot.ID, ordinal, snapshot.Timestamp, p)
			s.incidentStates[key] = iState
			if iEvent != nil {
				s.incidents[iEvent.ID] = *iEvent
			}
			ordinal++
		}
	}
	s.addSnapshot(snapshot)
	return snapshot, nil
}

func NewMemoryStore(capacity int) *MemoryStore {
	if capacity < 1 {
		capacity = 1
	}

	return &MemoryStore{
		incidentStates: make(map[string]incident.State), incidents: make(map[string]incident.Event),
		anomalyStates: make(map[string]anomaly.State), anomalyEvents: make(map[string]anomaly.Event),
		snapshots: make([]monitor.Snapshot, 0, capacity),
		capacity:  capacity,
	}
}

func (s *MemoryStore) AddSnapshot(snapshot monitor.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addSnapshot(snapshot)
}

func (s *MemoryStore) addSnapshot(snapshot monitor.Snapshot) {

	if len(s.snapshots) >= s.capacity {
		s.snapshots = s.snapshots[1:]
	}

	s.snapshots = append(s.snapshots, cloneSnapshot(snapshot))
	// Closed detector records follow the test/development raw-history window.
	if len(s.snapshots) > 0 {
		oldest := s.snapshots[0].Timestamp
		for id, event := range s.incidents {
			if event.RecoveryCycle != nil && event.RecoveryCycle.Before(oldest) {
				delete(s.incidents, id)
			}
		}
		for key, state := range s.incidentStates {
			if state.Active == nil && state.LastObserved.Before(oldest) {
				delete(s.incidentStates, key)
			}
		}
		for id, event := range s.anomalyEvents {
			if event.RecoveryCycle != nil && event.RecoveryCycle.Before(oldest) {
				delete(s.anomalyEvents, id)
			}
		}
		for key, state := range s.anomalyStates {
			if state.Active == nil && state.LastObserved.Before(oldest) {
				delete(s.anomalyStates, key)
			}
		}
	}
}

func (s *MemoryStore) QuerySnapshots(ctx context.Context, q Query) (Page, error) {
	q, err := q.normalized()
	if err != nil {
		return Page{}, err
	}
	if err := s.Ping(ctx); err != nil {
		return Page{}, err
	}
	var before cursor
	if q.Before != "" {
		before, _ = decodeCursor(q.Before)
	}
	result := make([]monitor.Snapshot, 0)
	for _, snapshot := range s.GetSnapshots() {
		if !matchesTime(q.Filter, snapshot.Timestamp) {
			continue
		}
		if q.Before != "" && (formatTime(snapshot.Timestamp) > before.Timestamp || (formatTime(snapshot.Timestamp) == before.Timestamp && snapshot.ID >= before.ID)) {
			continue
		}
		if q.Type != "" || q.Target != "" {
			matched := false
			for _, probes := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
				for _, p := range probes {
					if matchesProbe(q.Filter, p) {
						matched = true
					}
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, snapshot)
	}
	return pageSnapshots(result, q), nil
}

func (s *MemoryStore) Stats(ctx context.Context, f Filter) (Statistics, error) {
	stats := Statistics{Probes: map[string]*ProbeStats{}}
	if err := f.Validate(); err != nil {
		return stats, err
	}
	if err := s.Ping(ctx); err != nil {
		return stats, err
	}
	for _, snapshot := range s.GetSnapshots() {
		if !matchesTime(f, snapshot.Timestamp) {
			continue
		}
		for _, probes := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
			for _, p := range probes {
				if !matchesProbe(f, p) {
					continue
				}
				st := stats.Probes[p.Type]
				if st == nil {
					st = &ProbeStats{}
					stats.Probes[p.Type] = st
				}
				st.Total++
				stats.TotalResults++
				if !p.Success {
					st.Failed++
					continue
				}
				st.Successful++
				st.AvgLatency += p.LatencyMs
				if st.Successful == 1 || p.LatencyMs < st.MinLatency {
					st.MinLatency = p.LatencyMs
				}
				if p.LatencyMs > st.MaxLatency {
					st.MaxLatency = p.LatencyMs
				}
			}
		}
	}
	for _, st := range stats.Probes {
		if st.Successful > 0 {
			st.AvgLatency /= float64(st.Successful)
		}
	}
	return stats, nil
}

func (s *MemoryStore) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, fmt.Errorf("store is closed")
	}
	kept := make([]monitor.Snapshot, 0, len(s.snapshots))
	var deleted int64
	for _, snapshot := range s.snapshots {
		if snapshot.Timestamp.Before(before) && deleted < CleanupBatchSize {
			deleted++
			continue
		}
		kept = append(kept, snapshot)
	}
	s.snapshots = kept
	return deleted, nil
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("store is closed")
	}
	return nil
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *MemoryStore) GetSnapshots() []monitor.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]monitor.Snapshot, len(s.snapshots))
	for i, snapshot := range s.snapshots {
		snapshots[i] = cloneSnapshot(snapshot)
	}

	return snapshots
}

// GetAll returns individual probe results across all snapshots.
func (s *MemoryStore) GetAll() []*monitor.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*monitor.ProbeResult, 0)

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

func cloneSnapshot(snapshot monitor.Snapshot) monitor.Snapshot {
	snapshot.TCP = append(make([]monitor.ProbeResult, 0, len(snapshot.TCP)), snapshot.TCP...)
	snapshot.DNS = append(make([]monitor.ProbeResult, 0, len(snapshot.DNS)), snapshot.DNS...)
	return snapshot
}

func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.snapshots)
}
