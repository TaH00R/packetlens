package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func sampleSnapshot(at time.Time) monitor.Snapshot {
	return monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{
		{Type: "tcp", Target: "127.0.0.1:443", Timestamp: at.Add(-time.Millisecond), Success: true, LatencyMs: 10},
		{Type: "tcp", Target: "127.0.0.1:80", Timestamp: at, LatencyMs: 3000, Error: "connection refused"},
	}, DNS: []monitor.ProbeResult{{Type: "dns", Target: "example.test", Timestamp: at, Success: true, LatencyMs: 20}}}
}

func openTestSQLite(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "space & path", "history #1.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLitePersistsAndMigrates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history #1.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 6, 12, 0, 0, 123456789, time.FixedZone("offset", 19800))
	want, err := s.SaveSnapshot(ctx, sampleSnapshot(at))
	if err != nil {
		t.Fatal(err)
	}
	if !validID(want.ID) {
		t.Fatalf("missing stable ID: %q", want.ID)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	page, err := s.QuerySnapshots(ctx, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Snapshots) != 1 {
		t.Fatalf("lost persisted cycle: %+v", page)
	}
	got := page.Snapshots[0]
	if got.ID != want.ID || !got.Timestamp.Equal(at) || len(got.TCP) != 2 || len(got.DNS) != 1 || got.TCP[1].Error != "connection refused" || !got.TCP[0].Timestamp.Equal(want.TCP[0].Timestamp) {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	var version, foreignKeys int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if version != 5 || foreignKeys != 1 {
		t.Fatalf("version=%d foreign_keys=%d", version, foreignKeys)
	}
	for _, name := range []string{"snapshots_timestamp_id", "probe_results_timestamp", "probe_results_target_type", "probe_results_type"} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("missing index %s: %v", name, err)
		}
	}
	// A newly opened physical connection must also enforce foreign keys.
	s.db.SetMaxIdleConns(0)
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("connection lost foreign keys: %d %v", foreignKeys, err)
	}
}

func TestStoreQueries(t *testing.T) {
	for name, factory := range map[string]func(*testing.T) Store{
		"sqlite": func(t *testing.T) Store { return openTestSQLite(t) },
		"memory": func(t *testing.T) Store { return NewMemoryStore(20) },
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := factory(t)
			t.Cleanup(func() { s.Close() })
			base := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
			// Equal timestamps have distinct identities and deterministic pagination.
			for i, delta := range []time.Duration{3, 1, 2, 2, 4} {
				snapshot := sampleSnapshot(base.Add(delta * time.Second))
				snapshot.ID = fmt.Sprintf("%032x", i+1)
				if _, err := s.SaveSnapshot(ctx, snapshot); err != nil {
					t.Fatal(err)
				}
			}
			from, to := base.Add(2*time.Second), base.Add(3*time.Second)
			q := Query{Filter: Filter{From: &from, To: &to}, Limit: 2}
			page, err := s.QuerySnapshots(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Snapshots) != 2 || page.NextCursor == "" || page.Snapshots[0].ID != fmt.Sprintf("%032x", 4) {
				t.Fatalf("first page: %+v", page)
			}
			seen := map[string]bool{}
			for _, snapshot := range page.Snapshots {
				seen[snapshot.ID] = true
			}
			// A new live cycle must not change the next older page.
			if _, err := s.SaveSnapshot(ctx, sampleSnapshot(base.Add(5*time.Second))); err != nil {
				t.Fatal(err)
			}
			q.Before = page.NextCursor
			page, err = s.QuerySnapshots(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Snapshots) != 1 || page.NextCursor != "" || seen[page.Snapshots[0].ID] || page.Snapshots[0].ID != fmt.Sprintf("%032x", 3) {
				t.Fatalf("second page: %+v", page)
			}
			page, err = s.QuerySnapshots(ctx, Query{Filter: Filter{Type: "dns", Target: "example.test"}})
			if err != nil || len(page.Snapshots) != 6 || len(page.Snapshots[0].TCP) != 2 {
				t.Fatalf("filter must preserve complete cycles: %+v %v", page, err)
			}
			page, err = s.QuerySnapshots(ctx, Query{Filter: Filter{Type: "tcp", Target: "missing"}})
			if err != nil || page.Snapshots == nil || len(page.Snapshots) != 0 {
				t.Fatalf("empty result: %+v %v", page, err)
			}
			stats, err := s.Stats(ctx, Filter{From: &from, To: &to, Type: "tcp"})
			if err != nil {
				t.Fatal(err)
			}
			want := ProbeStats{Total: 6, Successful: 3, Failed: 3, AvgLatency: 10, MinLatency: 10, MaxLatency: 10}
			if stats.TotalResults != 6 || *stats.Probes["tcp"] != want {
				t.Fatalf("statistics: %+v", stats)
			}
			failed, err := s.Stats(ctx, Filter{Target: "127.0.0.1:80"})
			if err != nil || failed.Probes["tcp"].AvgLatency != 0 || failed.Probes["tcp"].Failed != 6 {
				t.Fatalf("failed measurements counted as latency: %+v %v", failed, err)
			}
			for _, invalid := range []Query{{Limit: -1}, {Limit: MaxLimit + 1}, {Before: "bad"}, {Filter: Filter{From: &to, To: &from}}, {Filter: Filter{Type: "icmp"}}} {
				if _, err := s.QuerySnapshots(ctx, invalid); !errors.Is(err, ErrInvalidQuery) {
					t.Fatalf("accepted invalid query %+v: %v", invalid, err)
				}
			}
		})
	}
}

func TestSQLiteAtomicWrite(t *testing.T) {
	s := openTestSQLite(t)
	ctx := context.Background()
	// Fail after an earlier probe has been inserted, exercising actual rollback.
	_, err := s.db.Exec(`CREATE TRIGGER fail_dns BEFORE INSERT ON probe_results WHEN NEW.type = 'dns' BEGIN SELECT RAISE(ABORT, 'test write failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSnapshot(ctx, sampleSnapshot(time.Now())); err == nil {
		t.Fatal("expected write failure")
	}
	for _, table := range []string{"snapshots", "probe_results"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("partial transaction in %s: %d %v", table, n, err)
		}
	}
}

func TestSQLiteRetentionBatchesAndCascades(t *testing.T) {
	s := openTestSQLite(t)
	ctx := context.Background()
	cutoff := time.Now().UTC()
	old := sampleSnapshot(cutoff.Add(-time.Second))
	if _, err := s.SaveSnapshot(ctx, old); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: cutoff}); err != nil {
		t.Fatal(err)
	}
	// Seed more than a cleanup batch without one disk sync per fixture row.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < CleanupBatchSize; i++ {
		if _, err := tx.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?)", fmt.Sprintf("%032x", i+1), formatTime(old.Timestamp)); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n, err := s.DeleteBefore(ctx, cutoff); err != nil || n != CleanupBatchSize {
		t.Fatalf("batch n=%d err=%v", n, err)
	}
	if n, err := Cleanup(ctx, s, cutoff); err != nil || n != 1 {
		t.Fatalf("sweep n=%d err=%v", n, err)
	}
	page, err := s.QuerySnapshots(ctx, Query{})
	if err != nil || len(page.Snapshots) != 1 || !page.Snapshots[0].Timestamp.Equal(cutoff) || page.Snapshots[0].TCP == nil {
		t.Fatalf("retention boundary: %+v %v", page, err)
	}
	var probes int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM probe_results").Scan(&probes); err != nil || probes != 0 {
		t.Fatalf("orphan probes: %d %v", probes, err)
	}
}

func TestSQLiteConcurrentAccessAndCancellation(t *testing.T) {
	s := openTestSQLite(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Go(func() {
			for j := 0; j < 10; j++ {
				if _, err := s.SaveSnapshot(ctx, sampleSnapshot(time.Now())); err != nil {
					t.Error(err)
					return
				}
				if _, err := s.QuerySnapshots(ctx, Query{Limit: 3}); err != nil {
					t.Error(err)
					return
				}
				if _, err := s.Stats(ctx, Filter{}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	page, err := s.QuerySnapshots(ctx, Query{})
	if err != nil || len(page.Snapshots) != 40 {
		t.Fatalf("concurrent writes lost: %d %v", len(page.Snapshots), err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.QuerySnapshots(cancelled, Query{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled query: %v", err)
	}
	if _, err := s.SaveSnapshot(cancelled, sampleSnapshot(time.Now())); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled write: %v", err)
	}
	if _, err := Cleanup(cancelled, s, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled cleanup: %v", err)
	}
	s.Close()
	if _, err := s.QuerySnapshots(ctx, Query{}); err == nil {
		t.Fatal("closed database read succeeded")
	}
	if _, err := s.Stats(ctx, Filter{}); err == nil {
		t.Fatal("closed database statistics succeeded")
	}
}

func TestSQLiteRejectsCorruptAndFutureSchemas(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenSQLite(ctx, path); err == nil {
		s.Close()
		t.Fatal("corrupt database accepted")
	}
	path = filepath.Join(t.TempDir(), "future.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err := OpenSQLite(ctx, path); err == nil {
		s.Close()
		t.Fatal("future schema accepted")
	}
}

func TestSQLiteMigrationRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an incompatible preexisting table. Migration 1 must roll back
	// its earlier CREATE statements without deleting this existing data.
	if _, err := db.Exec("CREATE TABLE probe_results(marker TEXT); INSERT INTO probe_results VALUES ('preserve me')"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := OpenSQLite(context.Background(), path); err == nil {
		s.Close()
		t.Fatal("migration conflict accepted")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, snapshotTables int
	var marker string
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='snapshots'").Scan(&snapshotTables); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT marker FROM probe_results").Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if version != 0 || snapshotTables != 0 || marker != "preserve me" {
		t.Fatalf("migration was partially applied: version=%d tables=%d marker=%s", version, snapshotTables, marker)
	}
}

func TestUpgradeVersionOnePreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := migrations.ReadFile("migrations/001_snapshots.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?); PRAGMA user_version=1", fmt.Sprintf("%032x", 1), formatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	page, err := s.QuerySnapshots(context.Background(), Query{})
	if err != nil || len(page.Snapshots) != 1 {
		t.Fatalf("migration lost history: %+v %v", page, err)
	}
	feed, err := s.QueryAnomalies(context.Background(), AnomalyQuery{})
	if err != nil || len(feed.Events) != 0 {
		t.Fatal("migration invented retrospective anomalies")
	}
}

func TestUpgradeVersionTwoPreservesAnomalies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_snapshots.sql", "002_anomalies.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Now().UTC()
	snapshotID := fmt.Sprintf("%032x", 1)
	if _, err := db.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?)", snapshotID, formatTime(at)); err != nil {
		t.Fatal(err)
	}
	e := anomaly.Event{ID: snapshotID + "-0", Type: "tcp", Target: "a", Timestamp: at, CycleTimestamp: at, LastAnomalyAt: at, LastObservedAt: at, ObservedLatency: 100, PeakLatency: 100, Baseline: 10, MAD: 0, Threshold: 20, Severity: "critical", AnomalousSamples: 1, Method: "rolling-median-mad-v1", Policy: anomaly.DefaultConfig(), BaselineSamples: 20}
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO latency_anomalies(id,type,target,cycle_timestamp,document) VALUES (?,?,?,?,?)", e.ID, e.Type, e.Target, formatTime(at), string(body)); err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(anomaly.State{Active: &e, LastObserved: at})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO anomaly_state(type,target,last_observed,active_id,document) VALUES (?,?,?,?,?); PRAGMA user_version=2", e.Type, e.Target, formatTime(at), e.ID, string(state)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old, err := s.QueryAnomalies(context.Background(), AnomalyQuery{})
	if err != nil || len(old.Events) != 1 || old.Events[0].ID != e.ID {
		t.Fatal("incident migration changed existing anomaly history")
	}
	incidents, err := s.QueryIncidents(context.Background(), IncidentQuery{})
	if err != nil || len(incidents.Events) != 0 {
		t.Fatal("migration invented incident history")
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 5 {
		t.Fatal("incident migration was not recorded")
	}
}

type failingCleanupStore struct {
	Store
	calls int
}

func (s *failingCleanupStore) DeleteBefore(context.Context, time.Time) (int64, error) {
	s.calls++
	if s.calls == 1 {
		return CleanupBatchSize, nil
	}
	return 0, errors.New("database unavailable")
}

func TestCleanupStopsOnFailure(t *testing.T) {
	s := &failingCleanupStore{}
	n, err := Cleanup(context.Background(), s, time.Now())
	if err == nil || n != CleanupBatchSize || s.calls != 2 {
		t.Fatalf("cleanup did not stop after failure: n=%d calls=%d err=%v", n, s.calls, err)
	}
}
