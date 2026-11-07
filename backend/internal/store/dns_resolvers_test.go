package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func comparisonSnapshot(at time.Time) monitor.Snapshot {
	return monitor.Snapshot{Timestamp: at, DNS: []monitor.ProbeResult{{Type: "dns", Target: "example.test", Timestamp: at, Success: true, LatencyMs: 10}}, DNSComparisons: []monitor.DNSComparison{{ProbeResult: monitor.ProbeResult{Type: "dns", Target: "example.test", Timestamp: at, LatencyMs: 20, Success: true}, Resolver: "127.0.0.1:53", Outcome: "success"}}}
}

func TestUpgradeFivePreservesTargetsAndDNSHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_snapshots.sql", "002_anomalies.sql", "003_incidents.sql", "004_correlations.sql", "005_targets.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	at := formatTime(time.Now())
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = db.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?); INSERT INTO probe_results(snapshot_id,ordinal,type,target,latency_ms,success,timestamp) VALUES (?,0,'dns','example.test',10,1,?); UPDATE monitoring_targets SET enabled=0; PRAGMA user_version=5", id, at, id, at); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	page, err := s.QuerySnapshots(context.Background(), Query{})
	if err != nil || len(page.Snapshots) != 1 || len(page.Snapshots[0].DNS) != 1 || len(page.Snapshots[0].DNSComparisons) != 0 {
		t.Fatal("migration changed history", err)
	}
	targets, err := s.ListTargets(context.Background())
	if err != nil || targets[0].Enabled {
		t.Fatal("migration changed target settings", err)
	}
}
func TestResolverHistoryPersistenceIsolationAndRetention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "resolver.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	at := time.Now().UTC()
	saved, err := s.SaveSnapshot(ctx, comparisonSnapshot(at))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.QuerySnapshots(ctx, Query{})
	if err != nil || len(page.Snapshots) != 1 || len(page.Snapshots[0].DNSComparisons) != 1 || page.Snapshots[0].DNSComparisons[0].Resolver != "127.0.0.1:53" {
		t.Fatal("resolver hydration lost", page, err)
	}
	stats, err := s.Stats(ctx, Filter{})
	if err != nil || stats.TotalResults != 1 || stats.Probes["dns"].AvgLatency != 10 {
		t.Fatal("comparison contaminated existing stats", stats, err)
	}
	n := 0
	if err = s.WalkDNSComparisons(ctx, Filter{From: &at, To: &at}, func(cycle time.Time, p monitor.DNSComparison) error {
		n++
		if !cycle.Equal(at) || p.LatencyMs != 20 {
			t.Error(p)
		}
		return nil
	}); err != nil || n != 1 {
		t.Fatal("inclusive query", err, n)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_comparison BEFORE INSERT ON dns_resolver_results BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSnapshot(ctx, comparisonSnapshot(at.Add(time.Second))); err == nil {
		t.Fatal("partial comparison write committed")
	}
	page, _ = s.QuerySnapshots(ctx, Query{})
	if len(page.Snapshots) != 1 || page.Snapshots[0].ID != saved.ID {
		t.Fatal("partial raw cycle committed")
	}
	if _, err = s.DeleteBefore(ctx, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM dns_resolver_results").Scan(&count); err != nil || count != 0 {
		t.Fatal("retention did not cascade", err)
	}
}
func TestComparisonValidationAndMemoryIsolation(t *testing.T) {
	s := NewMemoryStore(10)
	at := time.Now()
	sample := comparisonSnapshot(at)
	if _, err := s.SaveSnapshot(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	sample.DNSComparisons[0].Resolver = "mutated"
	page, _ := s.QuerySnapshots(context.Background(), Query{})
	if page.Snapshots[0].DNSComparisons[0].Resolver == "mutated" {
		t.Fatal("caller mutated stored comparison")
	}
	for _, bad := range []string{"dns.example:53", "0.0.0.0:53"} {
		sample = comparisonSnapshot(at)
		sample.DNSComparisons[0].Resolver = bad
		if _, err := s.SaveSnapshot(context.Background(), sample); err == nil {
			t.Fatal("invalid endpoint stored")
		}
	}
	sample = comparisonSnapshot(at)
	sample.DNSComparisons[0].Outcome = "timeout"
	if _, err := s.SaveSnapshot(context.Background(), sample); err == nil {
		t.Fatal("success stored as timeout")
	}
}
