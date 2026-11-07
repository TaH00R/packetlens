package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func overlap(at time.Time, success bool) monitor.Snapshot {
	return monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at, Success: success}}, DNS: []monitor.ProbeResult{{Type: "dns", Target: "b", Timestamp: at, Success: success}}}
}
func TestCorrelationRestartRollbackRetention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "correlations.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	at := time.Now().UTC()
	if _, err = s.SaveSnapshot(ctx, overlap(at, false)); err != nil {
		t.Fatal(err)
	}
	page, err := s.QueryCorrelations(ctx, EpisodeQuery{})
	if err != nil || len(page.Events) != 1 {
		t.Fatal(page, err)
	}
	id := page.Events[0].ID
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_correlation BEFORE INSERT ON correlations BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSnapshot(ctx, overlap(at.Add(time.Second), false)); err == nil {
		t.Fatal("write failure hidden")
	}
	raw, _ := s.QuerySnapshots(ctx, Query{})
	if len(raw.Snapshots) != 1 {
		t.Fatal("partial raw write")
	}
	page, _ = s.QueryCorrelations(ctx, EpisodeQuery{})
	if !page.Events[0].LastCorrelatedAt.Equal(at) {
		t.Fatal("failed write advanced correlation")
	}
	if _, err = s.db.Exec("DROP TRIGGER fail_correlation"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSnapshot(ctx, overlap(at.Add(2*time.Second), false)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryCorrelations(ctx, EpisodeQuery{})
	if len(page.Events) != 1 || page.Events[0].ID != id {
		t.Fatal("restart duplicated event")
	}
	if _, err = Cleanup(ctx, s, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryCorrelations(ctx, EpisodeQuery{})
	if len(page.Events) != 1 {
		t.Fatal("active correlation deleted")
	}
	if _, err = s.SaveSnapshot(ctx, overlap(at.Add(2*time.Hour), true)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryCorrelations(ctx, EpisodeQuery{})
	if page.Events[0].EndedAt == nil {
		t.Fatal("missing end")
	}
	if _, err = Cleanup(ctx, s, at.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryCorrelations(ctx, EpisodeQuery{})
	if len(page.Events) != 0 {
		t.Fatal("expired event retained")
	}
	var members int
	if err = s.db.QueryRow("SELECT count(*) FROM correlation_members").Scan(&members); err != nil || members != 0 {
		t.Fatal("members not cascaded", err)
	}
	if _, err = s.db.Exec("UPDATE correlation_state SET document='{}'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSnapshot(ctx, overlap(at.Add(4*time.Hour), false)); err == nil {
		t.Fatal("invalid state ignored")
	}
}

func TestRecentFailureSurvivesRestartAndDuplicateSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recent.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	saveOutcome(t, s, at, "a", false)
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stamp := at.Add(time.Second)
	snapshot, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: stamp, DNS: []monitor.ProbeResult{{Type: "dns", Target: "b", Timestamp: stamp}}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.QueryCorrelations(ctx, EpisodeQuery{})
	if err != nil || len(page.Events) != 1 || len(page.Events[0].Members) != 2 {
		t.Fatal("recent failure lost on restart", page, err)
	}
	var before string
	if err := s.db.QueryRow("SELECT document FROM correlation_state").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSnapshot(ctx, snapshot); err == nil {
		t.Fatal("duplicate snapshot accepted")
	}
	var after string
	if err := s.db.QueryRow("SELECT document FROM correlation_state").Scan(&after); err != nil || before != after {
		t.Fatal("duplicate advanced correlation", err)
	}
}
func TestCorrelationQueriesAndConcurrency(t *testing.T) {
	for _, name := range []string{"sqlite", "memory"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var s Store
			if name == "memory" {
				s = NewMemoryStore(100)
			} else {
				db, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "test.db"))
				if err != nil {
					t.Fatal(err)
				}
				s = db
			}
			defer s.Close()
			at := time.Now().UTC()
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := s.SaveSnapshot(ctx, overlap(at, false)); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			page, err := s.QueryCorrelations(ctx, EpisodeQuery{})
			if err != nil || len(page.Events) != 1 {
				t.Fatal("concurrent duplicates", page, err)
			}
			for _, q := range []EpisodeQuery{{Filter: Filter{Type: "dns", Target: "a"}}, {Filter: Filter{Target: "missing"}}} {
				p, err := s.QueryCorrelations(ctx, q)
				if err != nil || len(p.Events) != 0 {
					t.Fatal("member filter mismatch", p, err)
				}
			}
			if _, err := s.SaveSnapshot(ctx, overlap(at.Add(time.Second), true)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.SaveSnapshot(ctx, overlap(at.Add(2*time.Second), false)); err != nil {
				t.Fatal(err)
			}
			page, err = s.QueryCorrelations(ctx, EpisodeQuery{Limit: 1, Filter: Filter{Type: "dns", Target: "b"}})
			if err != nil || len(page.Events) != 1 || page.NextCursor == "" {
				t.Fatal("pagination missing", page, err)
			}
			older, err := s.QueryCorrelations(ctx, EpisodeQuery{Limit: 1, Before: page.NextCursor})
			if err != nil || len(older.Events) != 1 || older.Events[0].ID == page.Events[0].ID || older.NextCursor != "" {
				t.Fatal("cursor repeated event", older, err)
			}
			from := at.Add(2 * time.Second)
			page, err = s.QueryCorrelations(ctx, EpisodeQuery{Filter: Filter{From: &from}})
			if err != nil || len(page.Events) != 1 {
				t.Fatal("overlap range mismatch", page, err)
			}
			for _, q := range []EpisodeQuery{{Limit: 201}, {Before: "bad"}, {Filter: Filter{Type: "icmp"}}} {
				if _, err := s.QueryCorrelations(ctx, q); err == nil {
					t.Fatal("invalid query accepted")
				}
			}
		})
	}
}
func TestUpgradeVersionThree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_snapshots.sql", "002_anomalies.sql", "003_incidents.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?); PRAGMA user_version=3", fmt.Sprintf("%032x", 1), formatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	raw, err := s.QuerySnapshots(context.Background(), Query{})
	if err != nil || len(raw.Snapshots) != 1 {
		t.Fatal("migration lost raw history", err)
	}
	p, err := s.QueryCorrelations(context.Background(), EpisodeQuery{})
	if err != nil || len(p.Events) != 0 {
		t.Fatal("retrospective correlations invented", err)
	}
}
