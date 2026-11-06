package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type AnomalyPage struct {
	Events     []anomaly.Event
	NextCursor string
}

func (s *SQLiteStore) processAnomaly(ctx context.Context, tx *sql.Tx, snapshot monitor.Snapshot, ordinal int, p monitor.ProbeResult) error {
	var previous anomaly.State
	var document string
	err := tx.QueryRowContext(ctx, "SELECT document FROM anomaly_state WHERE type=? AND target=?", p.Type, p.Target).Scan(&document)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load anomaly state: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal([]byte(document), &previous); err != nil {
			return fmt.Errorf("decode anomaly state for %s/%s: %w", p.Type, p.Target, err)
		}
		if err := previous.Validate(); err != nil {
			return fmt.Errorf("invalid persisted detector state: %w", err)
		}
		if previous.Active != nil && (!validEventID(previous.Active.ID) || previous.Active.Type != p.Type || previous.Active.Target != p.Target) {
			return fmt.Errorf("persisted detector target mismatch")
		}
	}
	next, event := anomaly.Process(previous, s.anomalyConfig, snapshot.ID, ordinal, snapshot.Timestamp, p)
	if event != nil {
		body, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode anomaly: %w", err)
		}
		var recovery any
		if event.RecoveryCycle != nil {
			recovery = formatTime(*event.RecoveryCycle)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO latency_anomalies(id,type,target,cycle_timestamp,recovery_cycle,document) VALUES (?,?,?,?,?,?)
   ON CONFLICT(id) DO UPDATE SET recovery_cycle=excluded.recovery_cycle, document=excluded.document`, event.ID, event.Type, event.Target, formatTime(event.CycleTimestamp), recovery, string(body))
		if err != nil {
			return fmt.Errorf("persist anomaly episode: %w", err)
		}
	}
	body, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode detector state: %w", err)
	}
	var active any
	if next.Active != nil {
		active = next.Active.ID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO anomaly_state(type,target,last_observed,active_id,document) VALUES (?,?,?,?,?)
  ON CONFLICT(type,target) DO UPDATE SET last_observed=excluded.last_observed, active_id=excluded.active_id, document=excluded.document`, p.Type, p.Target, formatTime(next.LastObserved), active, string(body))
	if err != nil {
		return fmt.Errorf("persist anomaly state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryAnomalies(ctx context.Context, q AnomalyQuery) (AnomalyPage, error) {
	page := AnomalyPage{Events: []anomaly.Event{}}
	q, err := q.validate()
	if err != nil {
		return page, err
	}
	where, args := episodeConditions(q.Filter, true)
	if q.Before != "" {
		c, _ := readEpisodeCursor(q.Before)
		where += " AND (cycle_timestamp,id)<(?,?)"
		args = append(args, c.Timestamp, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM latency_anomalies WHERE "+where+" ORDER BY cycle_timestamp DESC,id DESC LIMIT ?", args...)
	if err != nil {
		return page, fmt.Errorf("query anomalies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var body string
		var e anomaly.Event
		if err := rows.Scan(&body); err != nil {
			return page, err
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil {
			return page, fmt.Errorf("decode anomaly: %w", err)
		}
		if err := e.Validate(); err != nil || !validEventID(e.ID) {
			return page, fmt.Errorf("invalid persisted anomaly record")
		}
		page.Events = append(page.Events, e)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Events) > q.Limit {
		page.Events = page.Events[:q.Limit]
		page.NextCursor = episodeCursor(page.Events[len(page.Events)-1].CycleTimestamp, page.Events[len(page.Events)-1].ID)
	}
	return page, nil
}
func (s *SQLiteStore) WalkAnomalyStarts(ctx context.Context, f Filter, visit func(anomaly.Event) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	where, args := episodeConditions(f, false)
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM latency_anomalies WHERE "+where+" ORDER BY cycle_timestamp,id", args...)
	if err != nil {
		return fmt.Errorf("query anomaly markers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var body string
		var e anomaly.Event
		if err := rows.Scan(&body); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil {
			return err
		}
		if err := e.Validate(); err != nil || !validEventID(e.ID) {
			return fmt.Errorf("invalid persisted anomaly record")
		}
		if err := visit(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *MemoryStore) QueryAnomalies(ctx context.Context, q AnomalyQuery) (AnomalyPage, error) {
	page := AnomalyPage{Events: []anomaly.Event{}}
	q, err := q.validate()
	if err != nil {
		return page, err
	}
	if err := s.Ping(ctx); err != nil {
		return page, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var before cursor
	if q.Before != "" {
		before, _ = readEpisodeCursor(q.Before)
	}
	for _, e := range s.anomalyEvents {
		if q.From != nil && e.RecoveryCycle != nil && e.RecoveryCycle.Before(*q.From) {
			continue
		}
		if q.To != nil && e.CycleTimestamp.After(*q.To) {
			continue
		}
		if (q.Type != "" && e.Type != q.Type) || (q.Target != "" && e.Target != q.Target) {
			continue
		}
		if q.Before != "" && (formatTime(e.CycleTimestamp) > before.Timestamp || (formatTime(e.CycleTimestamp) == before.Timestamp && e.ID >= before.ID)) {
			continue
		}
		page.Events = append(page.Events, e)
	}
	sort.Slice(page.Events, func(i, j int) bool {
		a, b := page.Events[i], page.Events[j]
		if a.CycleTimestamp.Equal(b.CycleTimestamp) {
			return a.ID > b.ID
		}
		return a.CycleTimestamp.After(b.CycleTimestamp)
	})
	if len(page.Events) > q.Limit {
		page.Events = page.Events[:q.Limit]
		page.NextCursor = episodeCursor(page.Events[len(page.Events)-1].CycleTimestamp, page.Events[len(page.Events)-1].ID)
	}
	return page, nil
}
func (s *MemoryStore) WalkAnomalyStarts(ctx context.Context, f Filter, visit func(anomaly.Event) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if err := s.Ping(ctx); err != nil {
		return err
	}
	s.mu.RLock()
	events := make([]anomaly.Event, 0, len(s.anomalyEvents))
	for _, e := range s.anomalyEvents {
		events = append(events, e)
	}
	s.mu.RUnlock()
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !matchesTime(f, e.CycleTimestamp) || (f.Type != "" && e.Type != f.Type) || (f.Target != "" && e.Target != f.Target) {
			continue
		}
		if err := visit(e); err != nil {
			return err
		}
	}
	return nil
}
