package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func saveDNSComparisons(ctx context.Context, tx *sql.Tx, snapshot monitor.Snapshot) error {
	if len(snapshot.DNSComparisons) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO dns_resolver_results(snapshot_id,ordinal,target,resolver,latency_ms,success,outcome,error,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, p := range snapshot.DNSComparisons {
		if _, err := stmt.ExecContext(ctx, snapshot.ID, i, p.Target, p.Resolver, p.LatencyMs, p.Success, p.Outcome, p.Error, formatTime(p.Timestamp)); err != nil {
			return fmt.Errorf("persist DNS comparison: %w", err)
		}
	}
	return nil
}
func hydrateDNSComparisons(ctx context.Context, tx *sql.Tx, page *Page) error {
	if len(page.Snapshots) == 0 {
		return nil
	}
	ids := []any{}
	positions := map[string]int{}
	for i, s := range page.Snapshots {
		ids = append(ids, s.ID)
		positions[s.ID] = i
	}
	rows, err := tx.QueryContext(ctx, `SELECT snapshot_id,target,resolver,latency_ms,success,outcome,error,timestamp FROM dns_resolver_results WHERE snapshot_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`) ORDER BY snapshot_id,ordinal`, ids...)
	if err != nil {
		return fmt.Errorf("hydrate DNS comparisons: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, stamp string
		var p monitor.DNSComparison
		p.Type = "dns"
		if err := rows.Scan(&id, &p.Target, &p.Resolver, &p.LatencyMs, &p.Success, &p.Outcome, &p.Error, &stamp); err != nil {
			return err
		}
		p.Timestamp, err = time.Parse(timestampLayout, stamp)
		if err != nil {
			return err
		}
		i := positions[id]
		page.Snapshots[i].DNSComparisons = append(page.Snapshots[i].DNSComparisons, p)
	}
	return rows.Err()
}
func (s *SQLiteStore) WalkDNSComparisons(ctx context.Context, f Filter, visit func(time.Time, monitor.DNSComparison) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	where, args := timeConditions(f)
	if f.Type != "" && f.Type != "dns" {
		return nil
	}
	if f.Target != "" {
		where += " AND p.target=?"
		args = append(args, f.Target)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT s.timestamp,p.target,p.resolver,p.latency_ms,p.success,p.outcome,p.error,p.timestamp FROM dns_resolver_results p JOIN snapshots s ON s.id=p.snapshot_id WHERE `+where+` ORDER BY s.timestamp,s.id,p.ordinal`, args...)
	if err != nil {
		return fmt.Errorf("query DNS comparison: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cycle, stamp string
		var p monitor.DNSComparison
		p.Type = "dns"
		if err := rows.Scan(&cycle, &p.Target, &p.Resolver, &p.LatencyMs, &p.Success, &p.Outcome, &p.Error, &stamp); err != nil {
			return err
		}
		p.Timestamp, err = time.Parse(timestampLayout, stamp)
		if err != nil {
			return err
		}
		at, err := time.Parse(timestampLayout, cycle)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(at, p); err != nil {
			return err
		}
	}
	return rows.Err()
}
func (s *MemoryStore) WalkDNSComparisons(ctx context.Context, f Filter, visit func(time.Time, monitor.DNSComparison) error) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if err := s.Ping(ctx); err != nil {
		return err
	}
	for _, snapshot := range s.GetSnapshots() {
		if !matchesTime(f, snapshot.Timestamp) {
			continue
		}
		for _, p := range snapshot.DNSComparisons {
			if err := ctx.Err(); err != nil {
				return err
			}
			if matchesProbe(f, p.ProbeResult) {
				if err := visit(snapshot.Timestamp, p); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
