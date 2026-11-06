package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func saveOutcome(t *testing.T, s Store, at time.Time, target string, success bool) {
	t.Helper()
	_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: target, Timestamp: at, Success: success, LatencyMs: 20, Error: "local test failure"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIncidentRestartRollbackAndRetention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "incidents.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	base := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	saveOutcome(t, s, base, "a", false)
	saveOutcome(t, s, base.Add(time.Second), "a", false)
	page, _ := s.QueryIncidents(ctx, IncidentQuery{})
	if len(page.Events) != 0 {
		t.Fatal("candidate exposed as an incident")
	}
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_incident BEFORE INSERT ON incidents BEGIN SELECT RAISE(ABORT,'injected incident failure'); END`); err != nil {
		t.Fatal(err)
	}
	at := base.Add(2 * time.Second)
	if _, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at}}}); err == nil {
		t.Fatal("incident write failure was hidden")
	}
	raw, _ := s.QuerySnapshots(ctx, Query{})
	if len(raw.Snapshots) != 2 {
		t.Fatal("failed incident write committed a partial raw cycle")
	}
	var document string
	if err := s.db.QueryRow("SELECT document FROM incident_state WHERE target='a'").Scan(&document); err != nil {
		t.Fatal(err)
	}
	var state incident.State
	if err := json.Unmarshal([]byte(document), &state); err != nil || state.Candidate.Failures != 2 {
		t.Fatal("failed transaction advanced the streak")
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_incident"); err != nil {
		t.Fatal(err)
	}
	saveOutcome(t, s, at, "a", false)
	page, _ = s.QueryIncidents(ctx, IncidentQuery{})
	if len(page.Events) != 1 || page.OpenCount != 1 || !page.Events[0].StartedAt.Equal(base) || !page.Events[0].DetectedAt.Equal(at) {
		t.Fatalf("restart lost candidate: %+v", page)
	}
	id := page.Events[0].ID
	saveOutcome(t, s, base.Add(3*time.Second), "a", true)
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	saveOutcome(t, s, base.Add(4*time.Second), "a", true)
	page, err = s.QueryIncidents(ctx, IncidentQuery{})
	if err != nil || len(page.Events) != 1 || page.Events[0].ID != id || page.OpenCount != 0 || page.RecoveredCount != 1 || page.Events[0].DurationMs == nil || *page.Events[0].DurationMs != 4000 {
		t.Fatalf("recovery restart mismatch: %+v %v", page, err)
	}
	for i := 5; i < 8; i++ {
		saveOutcome(t, s, base.Add(time.Duration(i)*time.Second), "a", false)
	}
	if _, err := Cleanup(ctx, s, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryIncidents(ctx, IncidentQuery{})
	if len(page.Events) != 1 || page.Events[0].RecoveredAt != nil || page.Events[0].ID == id {
		t.Fatal("retention deleted open incident or kept expired recovery")
	}
	saveOutcome(t, s, base.Add(2*time.Hour), "a", true)
	saveOutcome(t, s, base.Add(2*time.Hour+time.Second), "a", true)
	page, _ = s.QueryIncidents(ctx, IncidentQuery{})
	if page.Events[0].RecoveredAt == nil || !page.Events[0].HasMonitoringGaps {
		t.Fatal("gap recovery metadata missing")
	}
	if _, err := Cleanup(ctx, s, base.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, _ = s.QueryIncidents(ctx, IncidentQuery{})
	if len(page.Events) != 0 {
		t.Fatal("expired recovered incident survived cleanup")
	}
}

func TestIncidentIdentityFiltersCountsAndPagination(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"sqlite", "memory"} {
		t.Run(name, func(t *testing.T) {
			var s Store
			if name == "sqlite" {
				s = openTestSQLite(t)
			} else {
				s = NewMemoryStore(100)
			}
			base := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
			for i := 0; i < 3; i++ {
				at := base.Add(time.Duration(i) * time.Second)
				_, err := s.SaveSnapshot(ctx, monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at}, {Type: "tcp", Target: "b", Timestamp: at}}, DNS: []monitor.ProbeResult{{Type: "dns", Target: "a", Timestamp: at}}})
				if err != nil {
					t.Fatal(err)
				}
			}
			for i := 3; i < 5; i++ {
				saveOutcome(t, s, base.Add(time.Duration(i)*time.Second), "a", true)
			}
			first, err := s.QueryIncidents(ctx, IncidentQuery{EpisodeQuery: EpisodeQuery{Limit: 1}})
			if err != nil || len(first.Events) != 1 || first.OpenCount != 2 || first.RecoveredCount != 1 || first.NextCursor == "" {
				t.Fatalf("first page: %+v %v", first, err)
			}
			next, err := s.QueryIncidents(ctx, IncidentQuery{EpisodeQuery: EpisodeQuery{Limit: 1, Before: first.NextCursor}})
			if err != nil || len(next.Events) != 1 || next.Events[0].ID == first.Events[0].ID || next.OpenCount != 2 {
				t.Fatal("pagination or whole-range counts changed")
			}
			from, to := base.Add(time.Hour), base.Add(2*time.Hour)
			open, err := s.QueryIncidents(ctx, IncidentQuery{EpisodeQuery: EpisodeQuery{Filter: Filter{From: &from, To: &to}}, Status: "open"})
			if err != nil || len(open.Events) != 2 {
				t.Fatal("old open incidents must overlap later windows")
			}
			recovered, err := s.QueryIncidents(ctx, IncidentQuery{Status: "recovered"})
			if err != nil || len(recovered.Events) != 1 || recovered.OpenCount != 2 {
				t.Fatal("status filter must not hide total open count")
			}
			filtered, err := s.QueryIncidents(ctx, IncidentQuery{EpisodeQuery: EpisodeQuery{Filter: Filter{Type: "dns", Target: "a"}}})
			if err != nil || len(filtered.Events) != 1 || filtered.Events[0].Type != "dns" {
				t.Fatal("probe identities mixed")
			}
			for _, q := range []IncidentQuery{{Status: "bad"}, {EpisodeQuery: EpisodeQuery{Limit: 201}}, {EpisodeQuery: EpisodeQuery{Before: "bad"}}} {
				if _, err := s.QueryIncidents(ctx, q); err == nil {
					t.Fatal("invalid query accepted")
				}
			}
			s.Close()
			if _, err := s.QueryIncidents(ctx, IncidentQuery{}); err == nil {
				t.Fatal("closed store succeeded")
			}
		})
	}
}

func TestCorruptIncidentStateRollsBackSnapshot(t *testing.T) {
	s := openTestSQLite(t)
	at := time.Now().UTC()
	saveOutcome(t, s, at, "a", false)
	if _, err := s.db.Exec("UPDATE incident_state SET document='{}'"); err != nil {
		t.Fatal(err)
	}
	_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at.Add(time.Second), TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at.Add(time.Second)}}})
	if err == nil {
		t.Fatal("corrupt incident state was silently replaced")
	}
	page, _ := s.QuerySnapshots(context.Background(), Query{})
	if len(page.Snapshots) != 1 {
		t.Fatal("partial cycle committed")
	}
}

func TestConcurrentFailuresCreateOneIncident(t *testing.T) {
	s := openTestSQLite(t)
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at}}})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	page, err := s.QueryIncidents(context.Background(), IncidentQuery{})
	if err != nil || len(page.Events) != 1 || page.OpenCount != 1 || page.Events[0].FailedProbes != 20 {
		t.Fatalf("concurrent failure state diverged: %+v %v", page, err)
	}
	duplicate := monitor.Snapshot{ID: "ffffffffffffffffffffffffffffffff", Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at}}}
	if _, err := s.SaveSnapshot(context.Background(), duplicate); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSnapshot(context.Background(), duplicate); err == nil {
		t.Fatal("duplicate snapshot was processed twice")
	}
	page, _ = s.QueryIncidents(context.Background(), IncidentQuery{})
	if page.Events[0].FailedProbes != 21 {
		t.Fatal("failed duplicate insert advanced the incident")
	}
}
