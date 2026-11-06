package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

//go:embed migrations/*.sql
var migrations embed.FS

type SQLiteStore struct{ db *sql.DB }

var _ Store = (*SQLiteStore)(nil)

// OpenSQLite accepts a filesystem path, not a caller-supplied SQLite DSN.
func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "synchronous(FULL)")
	params.Add("_pragma", "temp_store(FILE)")
	u.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// Analytics readers must not occupy the collector's only connection. WAL
	// permits these bounded concurrent readers alongside collection and cleanup.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	s := &SQLiteStore{db: db}
	if err := s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) initialize(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("database did not enable WAL")
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > len(entries) {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, len(entries))
	}
	for i := version; i < len(entries); i++ {
		body, err := migrations.ReadFile("migrations/" + entries[i].Name())
		if err != nil {
			return fmt.Errorf("read migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", i+1)); err != nil {
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveSnapshot(ctx context.Context, snapshot monitor.Snapshot) (monitor.Snapshot, error) {
	snapshot, err := prepareSnapshot(snapshot)
	if err != nil {
		return snapshot, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return snapshot, fmt.Errorf("begin snapshot write: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO snapshots(id, timestamp) VALUES (?, ?)", snapshot.ID, formatTime(snapshot.Timestamp)); err != nil {
		return snapshot, fmt.Errorf("insert snapshot: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO probe_results(snapshot_id, ordinal, type, target, latency_ms, success, error, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return snapshot, fmt.Errorf("prepare probe insert: %w", err)
	}
	defer stmt.Close()
	ordinal := 0
	for _, probes := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
		for _, p := range probes {
			if _, err := stmt.ExecContext(ctx, snapshot.ID, ordinal, p.Type, p.Target, p.LatencyMs, p.Success, p.Error, formatTime(p.Timestamp)); err != nil {
				return snapshot, fmt.Errorf("insert probe result: %w", err)
			}
			ordinal++
		}
	}
	if err := tx.Commit(); err != nil {
		return snapshot, fmt.Errorf("commit snapshot: %w", err)
	}
	return snapshot, nil
}

func timeConditions(f Filter) (string, []any) {
	where := "1=1"
	args := []any{}
	if f.From != nil {
		where += " AND s.timestamp >= ?"
		args = append(args, formatTime(*f.From))
	}
	if f.To != nil {
		where += " AND s.timestamp <= ?"
		args = append(args, formatTime(*f.To))
	}
	return where, args
}

func probeConditions(f Filter) (string, []any) {
	where := ""
	args := []any{}
	if f.Type != "" {
		where += " AND p.type = ?"
		args = append(args, f.Type)
	}
	if f.Target != "" {
		where += " AND p.target = ?"
		args = append(args, f.Target)
	}
	return where, args
}

func (s *SQLiteStore) QuerySnapshots(ctx context.Context, q Query) (Page, error) {
	q, err := q.normalized()
	if err != nil {
		return Page{}, err
	}
	where, args := timeConditions(q.Filter)
	if q.Type != "" || q.Target != "" {
		condition, values := probeConditions(q.Filter)
		where += " AND EXISTS (SELECT 1 FROM probe_results p WHERE p.snapshot_id = s.id" + condition + ")"
		args = append(args, values...)
	}
	if q.Before != "" {
		c, _ := decodeCursor(q.Before)
		where += " AND (s.timestamp, s.id) < (?, ?)"
		args = append(args, c.Timestamp, c.ID)
	}
	args = append(args, q.Limit+1)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Page{}, fmt.Errorf("begin history read: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT s.id, s.timestamp FROM snapshots s WHERE "+where+" ORDER BY s.timestamp DESC, s.id DESC LIMIT ?", args...)
	if err != nil {
		return Page{}, fmt.Errorf("query snapshots: %w", err)
	}
	snapshots := make([]monitor.Snapshot, 0)
	for rows.Next() {
		var snapshot monitor.Snapshot
		var timestamp string
		if err := rows.Scan(&snapshot.ID, &timestamp); err != nil {
			rows.Close()
			return Page{}, fmt.Errorf("scan snapshot: %w", err)
		}
		snapshot.Timestamp, err = time.Parse(timestampLayout, timestamp)
		if err != nil {
			rows.Close()
			return Page{}, fmt.Errorf("parse stored snapshot timestamp: %w", err)
		}
		snapshot.TCP = []monitor.ProbeResult{}
		snapshot.DNS = []monitor.ProbeResult{}
		snapshots = append(snapshots, snapshot)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return Page{}, fmt.Errorf("read snapshots: %w", rowErr)
	}
	page := pageSnapshots(snapshots, q)
	if len(page.Snapshots) > 0 {
		ids := make([]any, len(page.Snapshots))
		positions := make(map[string]int, len(ids))
		for i, snapshot := range page.Snapshots {
			ids[i] = snapshot.ID
			positions[snapshot.ID] = i
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		rows, err := tx.QueryContext(ctx, "SELECT snapshot_id, type, target, latency_ms, success, error, timestamp FROM probe_results WHERE snapshot_id IN ("+placeholders+") ORDER BY snapshot_id, ordinal", ids...)
		if err != nil {
			return Page{}, fmt.Errorf("query snapshot probes: %w", err)
		}
		for rows.Next() {
			var id, timestamp string
			var p monitor.ProbeResult
			if err := rows.Scan(&id, &p.Type, &p.Target, &p.LatencyMs, &p.Success, &p.Error, &timestamp); err != nil {
				rows.Close()
				return Page{}, fmt.Errorf("scan probe: %w", err)
			}
			p.Timestamp, err = time.Parse(timestampLayout, timestamp)
			if err != nil {
				rows.Close()
				return Page{}, fmt.Errorf("parse stored probe timestamp: %w", err)
			}
			snapshot := &page.Snapshots[positions[id]]
			if p.Type == "tcp" {
				snapshot.TCP = append(snapshot.TCP, p)
			} else {
				snapshot.DNS = append(snapshot.DNS, p)
			}
		}
		rowErr := rows.Err()
		rows.Close()
		if rowErr != nil {
			return Page{}, fmt.Errorf("read probes: %w", rowErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return Page{}, fmt.Errorf("finish history read: %w", err)
	}
	return page, nil
}

func (s *SQLiteStore) Stats(ctx context.Context, f Filter) (Statistics, error) {
	stats := Statistics{Probes: map[string]*ProbeStats{}}
	if err := f.Validate(); err != nil {
		return stats, err
	}
	where, args := timeConditions(f)
	condition, values := probeConditions(f)
	args = append(args, values...)
	rows, err := s.db.QueryContext(ctx, `SELECT p.type, COUNT(*), SUM(p.success),
		COALESCE(AVG(CASE WHEN p.success = 1 THEN p.latency_ms END), 0),
		COALESCE(MIN(CASE WHEN p.success = 1 THEN p.latency_ms END), 0),
		COALESCE(MAX(CASE WHEN p.success = 1 THEN p.latency_ms END), 0)
		FROM probe_results p JOIN snapshots s ON s.id = p.snapshot_id WHERE `+where+condition+" GROUP BY p.type", args...)
	if err != nil {
		return stats, fmt.Errorf("query statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var p ProbeStats
		if err := rows.Scan(&typ, &p.Total, &p.Successful, &p.AvgLatency, &p.MinLatency, &p.MaxLatency); err != nil {
			return stats, fmt.Errorf("scan statistics: %w", err)
		}
		p.Failed = p.Total - p.Successful
		stats.TotalResults += p.Total
		stats.Probes[typ] = &p
	}
	if err := rows.Err(); err != nil {
		return stats, fmt.Errorf("read statistics: %w", err)
	}
	return stats, nil
}

// DeleteBefore bounds each cleanup transaction and cascades probe deletion.
// Rows exactly at the retention boundary are retained.
func (s *SQLiteStore) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE id IN
		(SELECT id FROM snapshots WHERE timestamp < ? ORDER BY timestamp, id LIMIT ?)`, formatTime(before), CleanupBatchSize)
	if err != nil {
		return 0, fmt.Errorf("delete expired snapshots: %w", err)
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *SQLiteStore) Close() error                   { return s.db.Close() }
