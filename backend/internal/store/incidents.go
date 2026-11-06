package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type IncidentQuery struct {
	EpisodeQuery
	Status string
}
type IncidentPage struct {
	Events         []incident.Event
	NextCursor     string
	OpenCount      int
	RecoveredCount int
}

func (q IncidentQuery) validate() (IncidentQuery, error) {
	normalized, err := q.EpisodeQuery.validate()
	q.EpisodeQuery = normalized
	if err != nil {
		return q, err
	}
	if q.Status != "" && q.Status != "all" && q.Status != "open" && q.Status != "recovered" {
		return q, fmt.Errorf("%w: incident status must be all, open, or recovered", ErrInvalidQuery)
	}
	return q, nil
}

func (s *SQLiteStore) processIncident(ctx context.Context, tx *sql.Tx, snapshot monitor.Snapshot, ordinal int, p monitor.ProbeResult) error {
	var previous incident.State
	var document string
	err := tx.QueryRowContext(ctx, "SELECT document FROM incident_state WHERE type=? AND target=?", p.Type, p.Target).Scan(&document)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load incident state: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal([]byte(document), &previous); err != nil {
			return fmt.Errorf("decode incident state: %w", err)
		}
		if err := previous.Validate(); err != nil {
			return fmt.Errorf("invalid persisted incident state: %w", err)
		}
		if c := previous.Candidate; c != nil {
			if !validEventID(c.ID) || c.Type != p.Type || c.Target != p.Target {
				return fmt.Errorf("incident candidate identity mismatch")
			}
		}
		if e := previous.Active; e != nil {
			if !validEventID(e.ID) || e.Type != p.Type || e.Target != p.Target {
				return fmt.Errorf("active incident identity mismatch")
			}
		}
	}
	next, event := incident.Process(previous, s.incidentConfig, snapshot.ID, ordinal, snapshot.Timestamp, p)
	if event != nil {
		body, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode incident: %w", err)
		}
		var recovery any
		if event.RecoveryCycle != nil {
			recovery = formatTime(*event.RecoveryCycle)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO incidents(id,type,target,cycle_timestamp,recovery_cycle,document) VALUES (?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET recovery_cycle=excluded.recovery_cycle,document=excluded.document`, event.ID, event.Type, event.Target, formatTime(event.CycleTimestamp), recovery, string(body)); err != nil {
			return fmt.Errorf("persist incident: %w", err)
		}
	}
	body, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode incident state: %w", err)
	}
	var active any
	if next.Active != nil {
		active = next.Active.ID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO incident_state(type,target,last_observed,active_id,document) VALUES (?,?,?,?,?)
		ON CONFLICT(type,target) DO UPDATE SET last_observed=excluded.last_observed,active_id=excluded.active_id,document=excluded.document`, p.Type, p.Target, formatTime(next.LastObserved), active, string(body)); err != nil {
		return fmt.Errorf("persist incident state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryIncidents(ctx context.Context, q IncidentQuery) (IncidentPage, error) {
	page := IncidentPage{Events: []incident.Event{}}
	q, err := q.validate()
	if err != nil {
		return page, err
	}
	where, args := episodeConditions(q.Filter, true)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, fmt.Errorf("begin incident read: %w", err)
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN recovery_cycle IS NULL THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN recovery_cycle IS NOT NULL THEN 1 ELSE 0 END),0) FROM incidents WHERE `+where, args...).Scan(&page.OpenCount, &page.RecoveredCount); err != nil {
		return page, fmt.Errorf("count incidents: %w", err)
	}
	if q.Status == "open" {
		where += " AND recovery_cycle IS NULL"
	}
	if q.Status == "recovered" {
		where += " AND recovery_cycle IS NOT NULL"
	}
	if q.Before != "" {
		c, _ := readEpisodeCursor(q.Before)
		where += " AND (cycle_timestamp,id)<(?,?)"
		args = append(args, c.Timestamp, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := tx.QueryContext(ctx, "SELECT document FROM incidents WHERE "+where+" ORDER BY cycle_timestamp DESC,id DESC LIMIT ?", args...)
	if err != nil {
		return page, fmt.Errorf("query incidents: %w", err)
	}
	for rows.Next() {
		var body string
		var e incident.Event
		if err := rows.Scan(&body); err != nil {
			rows.Close()
			return page, err
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil {
			rows.Close()
			return page, fmt.Errorf("decode incident: %w", err)
		}
		if err := e.Validate(); err != nil || !validEventID(e.ID) {
			rows.Close()
			return page, fmt.Errorf("invalid persisted incident record")
		}
		page.Events = append(page.Events, e)
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return page, readErr
	}
	if len(page.Events) > q.Limit {
		page.Events = page.Events[:q.Limit]
		last := page.Events[len(page.Events)-1]
		page.NextCursor = episodeCursor(last.CycleTimestamp, last.ID)
	}
	if err := tx.Commit(); err != nil {
		return page, err
	}
	return page, nil
}

func (s *MemoryStore) QueryIncidents(ctx context.Context, q IncidentQuery) (IncidentPage, error) {
	page := IncidentPage{Events: []incident.Event{}}
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
	for _, e := range s.incidents {
		if q.From != nil && e.RecoveryCycle != nil && e.RecoveryCycle.Before(*q.From) {
			continue
		}
		if q.To != nil && e.CycleTimestamp.After(*q.To) {
			continue
		}
		if (q.Type != "" && e.Type != q.Type) || (q.Target != "" && e.Target != q.Target) {
			continue
		}
		if e.RecoveredAt == nil {
			page.OpenCount++
		} else {
			page.RecoveredCount++
		}
		if (q.Status == "open" && e.RecoveredAt != nil) || (q.Status == "recovered" && e.RecoveredAt == nil) {
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
		last := page.Events[len(page.Events)-1]
		page.NextCursor = episodeCursor(last.CycleTimestamp, last.ID)
	}
	return page, nil
}
