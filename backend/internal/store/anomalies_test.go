package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func anomalyConfig() anomaly.Config {
	c := anomaly.DefaultConfig()
	c.MinSamples = 3
	c.RecoverySamples = 2
	return c
}
func saveLatency(t *testing.T, s Store, at time.Time, target string, latency float64, success bool) {
	t.Helper()
	_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: target, Timestamp: at, LatencyMs: latency, Success: success}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAnomalyRestartAndAtomicity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "anomalies.db")
	cfg := anomalyConfig()
	s, err := OpenSQLiteWithAnomalies(ctx, path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	base := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		saveLatency(t, s, base.Add(time.Duration(i)*time.Second), "a", 10, true)
	}
	// The same transaction must roll back both the snapshot and detector updates.
	if _, err := s.db.Exec(`CREATE TRIGGER fail_anomaly BEFORE INSERT ON latency_anomalies BEGIN SELECT RAISE(ABORT,'test anomaly failure'); END`); err != nil {
		t.Fatal(err)
	}
	at := base.Add(3 * time.Second)
	if _, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at, LatencyMs: 21, Success: true}}}); err == nil {
		t.Fatal("injected anomaly write did not fail")
	}
	page, _ := s.QuerySnapshots(ctx, Query{})
	if len(page.Snapshots) != 3 {
		t.Fatal("partial snapshot committed")
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_anomaly"); err != nil {
		t.Fatal(err)
	}
	saveLatency(t, s, at, "a", 21, true)
	first, err := s.QueryAnomalies(ctx, AnomalyQuery{})
	if err != nil || len(first.Events) != 1 {
		t.Fatalf("missing anomaly: %+v %v", first, err)
	}
	id := first.Events[0].ID
	s.Close()
	s, err = OpenSQLiteWithAnomalies(ctx, path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	saveLatency(t, s, base.Add(4*time.Second), "a", 50, true)
	saveLatency(t, s, base.Add(5*time.Second), "a", 10, true)
	s.Close()
	s, err = OpenSQLiteWithAnomalies(ctx, path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	saveLatency(t, s, base.Add(6*time.Second), "a", 10, true)
	current, err := s.QueryAnomalies(ctx, AnomalyQuery{})
	if err != nil || len(current.Events) != 1 || current.Events[0].ID != id || current.Events[0].RecoveredAt == nil || current.Events[0].AnomalousSamples != 2 || current.Events[0].PeakLatency != 50 {
		t.Fatalf("restart continuity: %+v %v", current, err)
	}
	saveLatency(t, s, base.Add(7*time.Second), "a", 60, true)
	current, _ = s.QueryAnomalies(ctx, AnomalyQuery{})
	if len(current.Events) != 2 || current.Events[0].ID == id {
		t.Fatal("new episode after recovery was suppressed")
	}
	// Open records survive raw-snapshot retention; no recovery is inferred.
	if _, err := Cleanup(ctx, s, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	current, _ = s.QueryAnomalies(ctx, AnomalyQuery{})
	if len(current.Events) != 1 || current.Events[0].RecoveredAt != nil {
		t.Fatal("retention removed an unresolved episode")
	}
}

func TestAnomalyPaginationFiltersAndMarkers(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLiteWithAnomalies(ctx, filepath.Join(t.TempDir(), "feed.db"), anomalyConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	for _, target := range []string{"a", "b"} {
		for i := 0; i < 3; i++ {
			saveLatency(t, s, base.Add(time.Duration(i)*time.Second), target, 10, true)
		}
		saveLatency(t, s, base.Add(4*time.Second), target, 100, true)
	}
	first, err := s.QueryAnomalies(ctx, AnomalyQuery{Limit: 1})
	if err != nil || len(first.Events) != 1 || first.NextCursor == "" {
		t.Fatalf("first page %+v %v", first, err)
	}
	next, err := s.QueryAnomalies(ctx, AnomalyQuery{Limit: 1, Before: first.NextCursor})
	if err != nil || len(next.Events) != 1 || next.NextCursor != "" || first.Events[0].ID == next.Events[0].ID {
		t.Fatalf("second page %+v %v", next, err)
	}
	from := base.Add(time.Minute)
	to := base.Add(2 * time.Minute)
	filtered, err := s.QueryAnomalies(ctx, AnomalyQuery{Filter: Filter{From: &from, To: &to, Target: "a"}})
	if err != nil || len(filtered.Events) != 1 {
		t.Fatal("open episode overlap was lost")
	}
	starts := 0
	if err := s.WalkAnomalyStarts(ctx, Filter{From: &from, To: &to}, func(anomaly.Event) error { starts++; return nil }); err != nil || starts != 0 {
		t.Fatal("old ongoing episode was marked as a new onset")
	}
	for _, q := range []AnomalyQuery{{Limit: 201}, {Before: "invalid"}, {Filter: Filter{Type: "icmp"}}} {
		if _, err := s.QueryAnomalies(ctx, q); err == nil {
			t.Fatalf("accepted invalid query: %s", fmt.Sprint(q))
		}
	}
}

func TestCorruptDetectorStateDoesNotCommitRawHistory(t *testing.T) {
	s := openTestSQLite(t)
	ctx := context.Background()
	at := time.Now().UTC()
	saveLatency(t, s, at, "a", 10, true)
	if _, err := s.db.Exec("UPDATE anomaly_state SET document='{}' WHERE type='tcp' AND target='a'"); err != nil {
		t.Fatal(err)
	}
	next := at.Add(time.Second)
	if _, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: next, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: next, LatencyMs: 11, Success: true}}}); err == nil {
		t.Fatal("invalid detector state was silently replaced")
	}
	page, err := s.QuerySnapshots(ctx, Query{})
	if err != nil || len(page.Snapshots) != 1 {
		t.Fatal("corrupt state allowed a partial cycle commit")
	}
}
