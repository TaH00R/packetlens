package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/target"
)

func TestTargetPersistenceRollbackAndEmptyRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "targets.db")
	s, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	list, err := s.ListTargets(ctx)
	if err != nil || len(list) != 4 {
		t.Fatal("defaults missing", err)
	}
	list[0].Enabled = false
	list[0].IntervalMs = 10000
	list[0].TimeoutMs = 5000
	if err = s.ReplaceTargets(ctx, list); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ListTargets(ctx)
	if err != nil || got[0] != list[0] {
		t.Fatal("settings lost", got, err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER reject_target BEFORE INSERT ON monitoring_targets BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceTargets(ctx, list[1:]); err == nil {
		t.Fatal("injected failure hidden")
	}
	got, err = s.ListTargets(ctx)
	if err != nil || len(got) != 4 || got[0] != list[0] {
		t.Fatal("partial config committed", err)
	}
	if _, err = s.db.Exec("DROP TRIGGER reject_target"); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceTargets(ctx, []target.Config{}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.ListTargets(ctx)
	if err != nil || len(got) != 0 {
		t.Fatal("deleted defaults restored", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.ListTargets(ctx); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestUpgradeVersionFourSeedsTargetsAndPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_snapshots.sql", "002_anomalies.sql", "003_incidents.sql", "004_correlations.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec("INSERT INTO snapshots(id,timestamp) VALUES (?,?); PRAGMA user_version=4", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", formatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	list, err := s.ListTargets(context.Background())
	if err != nil || len(list) != 4 {
		t.Fatal("upgrade defaults", err)
	}
	raw, err := s.QuerySnapshots(context.Background(), Query{})
	if err != nil || len(raw.Snapshots) != 1 {
		t.Fatal("upgrade lost history", err)
	}
}
