package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/TaH00R/packetlens/backend/internal/correlation"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type CorrelationPage struct {
	Events     []correlation.Event
	NextCursor string
}

func (s *SQLiteStore) processCorrelation(ctx context.Context, tx *sql.Tx, snapshot monitor.Snapshot) error {
	var previous correlation.State
	var document string
	err := tx.QueryRowContext(ctx, "SELECT document FROM correlation_state WHERE singleton=1").Scan(&document)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load correlation state: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal([]byte(document), &previous); err != nil {
			return fmt.Errorf("decode correlation state: %w", err)
		}
		if err := previous.Validate(); err != nil {
			return fmt.Errorf("invalid persisted correlation state: %w", err)
		}
		if previous.Active != nil && !validEventID(previous.Active.ID) {
			return fmt.Errorf("invalid persisted correlation identity")
		}
	}
	next, events, err := correlation.Process(previous, s.correlationConfig, snapshot)
	if err != nil {
		return err
	}
	for _, e := range events {
		body, err := json.Marshal(e)
		if err != nil {
			return err
		}
		var end any
		if e.EndCycle != nil {
			end = formatTime(*e.EndCycle)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO correlations(id,cycle_timestamp,recovery_cycle,document) VALUES (?,?,?,?) ON CONFLICT(id) DO UPDATE SET recovery_cycle=excluded.recovery_cycle,document=excluded.document`, e.ID, formatTime(e.CycleTimestamp), end, string(body)); err != nil {
			return fmt.Errorf("persist correlation: %w", err)
		}
		for _, m := range e.Members {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO correlation_members(event_id,type,target) VALUES (?,?,?)`, e.ID, m.Type, m.Target); err != nil {
				return fmt.Errorf("persist correlation member: %w", err)
			}
		}
	}
	body, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO correlation_state(singleton,document) VALUES (1,?) ON CONFLICT(singleton) DO UPDATE SET document=excluded.document`, string(body)); err != nil {
		return fmt.Errorf("persist correlation state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryCorrelations(ctx context.Context, q EpisodeQuery) (CorrelationPage, error) {
	page := CorrelationPage{Events: []correlation.Event{}}
	q, err := q.validate()
	if err != nil {
		return page, err
	}
	// A type/target filter must match the same member, rather than separate
	// members of the event. Keep membership indexed outside the JSON document.
	f := q.Filter
	f.Type = ""
	f.Target = ""
	where, args := episodeConditions(f, true)
	if q.Type != "" || q.Target != "" {
		where += " AND EXISTS (SELECT 1 FROM correlation_members m WHERE m.event_id=correlations.id"
		if q.Type != "" {
			where += " AND m.type=?"
			args = append(args, q.Type)
		}
		if q.Target != "" {
			where += " AND m.target=?"
			args = append(args, q.Target)
		}
		where += ")"
	}
	if q.Before != "" {
		c, _ := readEpisodeCursor(q.Before)
		where += " AND (cycle_timestamp,id)<(?,?)"
		args = append(args, c.Timestamp, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM correlations WHERE "+where+" ORDER BY cycle_timestamp DESC,id DESC LIMIT ?", args...)
	if err != nil {
		return page, fmt.Errorf("query correlations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var body string
		var e correlation.Event
		if err := rows.Scan(&body); err != nil {
			return page, err
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil {
			return page, fmt.Errorf("decode correlation: %w", err)
		}
		if e.Validate() != nil || !validEventID(e.ID) {
			return page, fmt.Errorf("invalid persisted correlation")
		}
		page.Events = append(page.Events, e)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	return finishCorrelationPage(page, q.Limit), nil
}
func finishCorrelationPage(page CorrelationPage, limit int) CorrelationPage {
	if len(page.Events) > limit {
		page.Events = page.Events[:limit]
		e := page.Events[len(page.Events)-1]
		page.NextCursor = episodeCursor(e.CycleTimestamp, e.ID)
	}
	return page
}
func (s *MemoryStore) QueryCorrelations(ctx context.Context, q EpisodeQuery) (CorrelationPage, error) {
	page := CorrelationPage{Events: []correlation.Event{}}
	q, err := q.validate()
	if err != nil {
		return page, err
	}
	if err := s.Ping(ctx); err != nil {
		return page, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.correlations {
		if (q.From != nil && e.EndCycle != nil && e.EndCycle.Before(*q.From)) || (q.To != nil && e.CycleTimestamp.After(*q.To)) {
			continue
		}
		matched := q.Type == "" && q.Target == ""
		for _, m := range e.Members {
			if (q.Type == "" || m.Type == q.Type) && (q.Target == "" || m.Target == q.Target) {
				matched = true
			}
		}
		if !matched {
			continue
		}
		if q.Before != "" {
			c, _ := readEpisodeCursor(q.Before)
			if formatTime(e.CycleTimestamp) > c.Timestamp || (formatTime(e.CycleTimestamp) == c.Timestamp && e.ID >= c.ID) {
				continue
			}
		}
		e.Members = append([]correlation.Member{}, e.Members...)
		page.Events = append(page.Events, e)
	}
	sort.Slice(page.Events, func(i, j int) bool {
		a, b := page.Events[i], page.Events[j]
		if a.CycleTimestamp.Equal(b.CycleTimestamp) {
			return a.ID > b.ID
		}
		return a.CycleTimestamp.After(b.CycleTimestamp)
	})
	return finishCorrelationPage(page, q.Limit), nil
}
