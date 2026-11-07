package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/incident"
)

func TestIncidentStreamOverlapFiltersAndStop(t *testing.T) {
	for _, name := range []string{"memory", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			var source Store
			if name == "memory" {
				source = NewMemoryStore(30)
			} else {
				s, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "incidents.db"))
				if err != nil {
					t.Fatal(err)
				}
				source = s
			}
			defer source.Close()
			at := time.Now().UTC().Add(-time.Hour)
			for i := 0; i < 5; i++ {
				saveOutcome(t, source, at.Add(time.Duration(i)*time.Second), "a:443", i >= 3)
			}
			for i := 10; i < 13; i++ {
				saveOutcome(t, source, at.Add(time.Duration(i)*time.Second), "b:443", false)
			}
			from := at.Add(3 * time.Second)
			to := at.Add(12 * time.Second)
			count := 0
			if err := source.WalkIncidents(context.Background(), Filter{From: &from, To: &to, Type: "tcp"}, func(e incident.Event) error {
				count++
				if count == 1 && e.Target != "a:443" {
					t.Fatal("stream not chronological")
				}
				return nil
			}); err != nil || count != 2 {
				t.Fatal("overlap selection lost incident", count, err)
			}
			stop := errors.New("consumer stopped")
			count = 0
			if err := source.WalkIncidents(context.Background(), Filter{Target: "b:443"}, func(e incident.Event) error {
				count++
				if e.RecoveredAt != nil {
					t.Fatal("open incident recovered in export")
				}
				return stop
			}); !errors.Is(err, stop) || count != 1 {
				t.Fatal("consumer stop ignored", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := source.WalkIncidents(ctx, Filter{}, func(incident.Event) error { t.Fatal("cancelled callback invoked"); return nil }); err == nil {
				t.Fatal("cancelled stream succeeded")
			}
		})
	}
}
