package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/TaH00R/packetlens/backend/internal/incident"
)

// WalkIncidents streams the current stored state of episodes overlapping the
// selected cycle-time window. A single SQLite read keeps the export consistent.
func (s *SQLiteStore) WalkIncidents(ctx context.Context, f Filter, visit func(incident.Event) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	where, args := episodeConditions(f, true)
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM incidents WHERE "+where+" ORDER BY cycle_timestamp,id", args...)
	if err != nil {
		return fmt.Errorf("stream incidents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var body string
		var event incident.Event
		if err := rows.Scan(&body); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(body), &event); err != nil {
			return fmt.Errorf("decode exported incident: %w", err)
		}
		if event.Validate() != nil || !validEventID(event.ID) {
			return fmt.Errorf("invalid persisted incident")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(event); err != nil {
			return err
		}
	}
	return rows.Err()
}
func (s *MemoryStore) WalkIncidents(ctx context.Context, f Filter, visit func(incident.Event) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if err := s.Ping(ctx); err != nil {
		return err
	}
	s.mu.RLock()
	events := []incident.Event{}
	for _, e := range s.incidents {
		if (f.From != nil && e.RecoveryCycle != nil && e.RecoveryCycle.Before(*f.From)) || (f.To != nil && e.CycleTimestamp.After(*f.To)) || (f.Type != "" && e.Type != f.Type) || (f.Target != "" && e.Target != f.Target) {
			continue
		}
		events = append(events, e)
	}
	s.mu.RUnlock()
	sort.Slice(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if a.CycleTimestamp.Equal(b.CycleTimestamp) {
			return a.ID < b.ID
		}
		return a.CycleTimestamp.Before(b.CycleTimestamp)
	})
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(e); err != nil {
			return err
		}
	}
	return nil
}
