package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

// Measurement ranks are one-based and count successful measurements only.
// Window ranks are calculated by SQLite, avoiding an unbounded slice in Go.
type Measurement struct {
	SnapshotTimestamp       time.Time
	Probe                   monitor.ProbeResult
	TypeRank, TypeCount     int
	TargetRank, TargetCount int
}

func (s *SQLiteStore) HistoryBounds(ctx context.Context) (*time.Time, *time.Time, error) {
	var first, last sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT timestamp FROM snapshots ORDER BY timestamp, id LIMIT 1),
		(SELECT timestamp FROM snapshots ORDER BY timestamp DESC, id DESC LIMIT 1)`).Scan(&first, &last); err != nil {
		return nil, nil, fmt.Errorf("query history bounds: %w", err)
	}
	if !first.Valid {
		return nil, nil, nil
	}
	a, err := time.Parse(timestampLayout, first.String)
	if err != nil {
		return nil, nil, err
	}
	b, err := time.Parse(timestampLayout, last.String)
	if err != nil {
		return nil, nil, err
	}
	return &a, &b, nil
}

func (s *SQLiteStore) WalkMeasurements(ctx context.Context, f Filter, ranked bool, visit func(Measurement) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	where, args := timeConditions(f)
	condition, values := probeConditions(f)
	args = append(args, values...)
	ranks := "0, 0, 0, 0"
	if ranked {
		ranks = `ROW_NUMBER() OVER (PARTITION BY p.type, p.success ORDER BY p.latency_ms, p.timestamp, s.timestamp, s.id, p.ordinal),
			SUM(p.success) OVER (PARTITION BY p.type),
			ROW_NUMBER() OVER (PARTITION BY p.type, p.target, p.success ORDER BY p.latency_ms, p.timestamp, s.timestamp, s.id, p.ordinal),
			SUM(p.success) OVER (PARTITION BY p.type, p.target)`
	}
	rows, err := s.db.QueryContext(ctx, `SELECT s.timestamp, p.type, p.target, p.latency_ms, p.success, p.error, p.timestamp, `+ranks+`
		FROM probe_results p JOIN snapshots s ON s.id=p.snapshot_id WHERE `+where+condition+`
		ORDER BY p.timestamp, s.timestamp, s.id, p.ordinal`, args...)
	if err != nil {
		return fmt.Errorf("query analytics measurements: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m Measurement
		var cycle, timestamp string
		p := &m.Probe
		if err := rows.Scan(&cycle, &p.Type, &p.Target, &p.LatencyMs, &p.Success, &p.Error, &timestamp, &m.TypeRank, &m.TypeCount, &m.TargetRank, &m.TargetCount); err != nil {
			return fmt.Errorf("scan analytics measurement: %w", err)
		}
		m.SnapshotTimestamp, err = time.Parse(timestampLayout, cycle)
		if err != nil {
			return err
		}
		p.Timestamp, err = time.Parse(timestampLayout, timestamp)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(m); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *MemoryStore) HistoryBounds(ctx context.Context) (*time.Time, *time.Time, error) {
	if err := s.Ping(ctx); err != nil {
		return nil, nil, err
	}
	var first, last *time.Time
	for _, snapshot := range s.GetSnapshots() {
		t := snapshot.Timestamp
		if first == nil || t.Before(*first) {
			first = &t
		}
		if last == nil || t.After(*last) {
			last = &t
		}
	}
	return first, last, nil
}

func (s *MemoryStore) WalkMeasurements(ctx context.Context, f Filter, ranked bool, visit func(Measurement) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if err := s.Ping(ctx); err != nil {
		return err
	}
	type entry struct {
		Measurement
		id      string
		ordinal int
	}
	entries := []entry{}
	for _, snapshot := range s.GetSnapshots() {
		if !matchesTime(f, snapshot.Timestamp) {
			continue
		}
		ordinal := 0
		for _, group := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
			for _, probe := range group {
				if matchesProbe(f, probe) {
					entries = append(entries, entry{Measurement: Measurement{SnapshotTimestamp: snapshot.Timestamp, Probe: probe}, id: snapshot.ID, ordinal: ordinal})
				}
				ordinal++
			}
		}
	}
	less := func(a, b entry) bool {
		if !a.Probe.Timestamp.Equal(b.Probe.Timestamp) {
			return a.Probe.Timestamp.Before(b.Probe.Timestamp)
		}
		if !a.SnapshotTimestamp.Equal(b.SnapshotTimestamp) {
			return a.SnapshotTimestamp.Before(b.SnapshotTimestamp)
		}
		if a.id != b.id {
			return a.id < b.id
		}
		return a.ordinal < b.ordinal
	}
	if ranked {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Probe.LatencyMs != entries[j].Probe.LatencyMs {
				return entries[i].Probe.LatencyMs < entries[j].Probe.LatencyMs
			}
			return less(entries[i], entries[j])
		})
		types, targets := map[string]int{}, map[string]int{}
		for i := range entries {
			e := &entries[i]
			if !e.Probe.Success {
				continue
			}
			types[e.Probe.Type]++
			targets[e.Probe.Type+":"+e.Probe.Target]++
			e.TypeRank = types[e.Probe.Type]
			e.TargetRank = targets[e.Probe.Type+":"+e.Probe.Target]
		}
		for i := range entries {
			entries[i].TypeCount = types[entries[i].Probe.Type]
			entries[i].TargetCount = targets[entries[i].Probe.Type+":"+entries[i].Probe.Target]
		}
	}
	sort.Slice(entries, func(i, j int) bool { return less(entries[i], entries[j]) })
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(e.Measurement); err != nil {
			return err
		}
	}
	return nil
}
