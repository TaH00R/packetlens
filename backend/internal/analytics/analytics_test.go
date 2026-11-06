package analytics_test

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/analytics"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func stores(t *testing.T, run func(*testing.T, store.Store)) {
	t.Helper()
	for _, name := range []string{"memory", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			var s store.Store
			if name == "memory" {
				s = store.NewMemoryStore(2000)
			} else {
				var err error
				s, err = store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "analytics.db"))
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(func() { s.Close() })
			run(t, s)
		})
	}
}

func add(t *testing.T, s store.Store, at time.Time, typ, target string, latency float64, success bool) {
	t.Helper()
	p := monitor.ProbeResult{Type: typ, Target: target, LatencyMs: latency, Success: success, Timestamp: at}
	snapshot := monitor.Snapshot{Timestamp: at}
	if typ == "tcp" {
		snapshot.TCP = []monitor.ProbeResult{p}
	} else {
		snapshot.DNS = []monitor.ProbeResult{p}
	}
	if _, err := s.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func near(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-8 {
		t.Fatalf("got %v, want %g", got, want)
	}
}

func TestNearestRank(t *testing.T) {
	for _, tc := range []struct{ n, p, rank int }{{0, 50, 0}, {1, 99, 1}, {5, 50, 3}, {5, 95, 5}, {100, 99, 99}, {4, 50, 2}, {5, 0, 0}} {
		if got := analytics.NearestRank(tc.n, tc.p); got != tc.rank {
			t.Fatalf("rank(%d,%d)=%d", tc.n, tc.p, got)
		}
	}
}

func TestEmptyAndAllFailure(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for _, selected := range []string{"5m", "15m", "1h", "6h", "24h", "all"} {
			r, err := analytics.Build(context.Background(), s, selected, now)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range r.Current.Groups {
				if g.Attempts != 0 || g.Average != nil || g.P50 != nil || g.Jitter != nil || g.Availability != nil {
					t.Fatalf("fabricated empty statistics: %+v", g)
				}
			}
			if selected == "all" && r.Previous != nil {
				t.Fatal("all must not invent a previous period")
			}
			for _, series := range r.Current.Trends {
				for _, p := range series.Points {
					if p.Availability != nil || p.Average != nil || p.Jitter != nil {
						t.Fatal("empty bucket became a zero measurement")
					}
				}
			}
		}
		add(t, s, now, "tcp", "failed", 3000, false)
		r, err := analytics.Build(context.Background(), s, "5m", now)
		if err != nil {
			t.Fatal(err)
		}
		g := r.Current.Groups[0]
		if g.Attempts != 1 || g.Failed != 1 || g.Average != nil || g.P99 != nil || g.Jitter != nil {
			t.Fatalf("failed probe counted as successful latency: %+v", g)
		}
		near(t, g.Availability, 0)
		a, err := analytics.Availability(context.Background(), s, now)
		if err != nil {
			t.Fatal(err)
		}
		if a.Groups[1].Windows[0].Availability != nil || a.Targets[0].ConsecutiveFailures != 1 {
			t.Fatalf("missing outcomes misrepresented: %+v", a)
		}
	})
}

func TestPercentilesHistogramAndTargetIsolation(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for i, value := range []float64{100, 10, 40, 20, 30} {
			add(t, s, now.Add(-time.Duration(i)*time.Second), "tcp", "a", value, true)
		}
		add(t, s, now, "tcp", "b", 1000, true)
		add(t, s, now, "dns", "a", 9999, true)
		add(t, s, now, "tcp", "a", 0, false)
		r, err := analytics.Build(context.Background(), s, "5m", now)
		if err != nil {
			t.Fatal(err)
		}
		near(t, r.Current.Groups[0].P50, 30)
		near(t, r.Current.Groups[0].P90, 1000)
		for _, target := range r.Current.Targets {
			if target.Type == "tcp" && target.Target == "a" {
				near(t, target.P50, 30)
				near(t, target.P90, 100)
				near(t, target.P95, 100)
				near(t, target.P99, 100)
				near(t, target.Average, 40)
				near(t, target.Minimum, 10)
				near(t, target.Maximum, 100)
				if target.Successful != 5 || target.Failed != 1 {
					t.Fatalf("counts: %+v", target)
				}
			}
		}
		count := 0
		for _, bin := range r.Current.Groups[0].Histogram {
			count += bin.Count
		}
		if count != 6 {
			t.Fatalf("histogram counted %d successes", count)
		}
		near(t, r.Current.Groups[1].P99, 9999)
	})
}

func TestHistogramBoundariesAndZeroLatency(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for i, latency := range []float64{0, 10, 25, 50, 100, 250, 500, 1000} {
			add(t, s, now.Add(-time.Duration(i)*time.Second), "tcp", "edges", latency, true)
		}
		r, err := analytics.Build(context.Background(), s, "5m", now)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range r.Current.Groups[0].Histogram {
			if b.Count != 1 {
				t.Fatalf("boundary landed in wrong bin: %+v", b)
			}
		}
		near(t, r.Current.Groups[0].Minimum, 0)
	})
}

func TestJitterChronologyFailuresAndCurrentPair(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for _, event := range []struct {
			seconds int
			latency float64
			success bool
		}{{40, 45, true}, {10, 20, true}, {30, 25, true}, {20, 30, true}, {25, 3000, false}, {45, 0, false}} {
			add(t, s, now.Add(-time.Minute+time.Duration(event.seconds)*time.Second), "tcp", "a", event.latency, event.success)
		}
		add(t, s, now.Add(-time.Second), "tcp", "b", 100, true)
		add(t, s, now, "tcp", "b", 200, true)
		add(t, s, now, "tcp", "single", 9999, true)
		r, err := analytics.Build(context.Background(), s, "5m", now)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range r.Current.Targets {
			switch target.Target {
			case "a":
				near(t, target.Jitter, 35.0/3)
				near(t, target.CurrentJitter, 20)
			case "b":
				near(t, target.Jitter, 100)
			case "single":
				if target.Jitter != nil || target.CurrentJitter != nil {
					t.Fatal("single success invented a pair")
				}
			}
		}
		near(t, r.Current.Groups[0].Jitter, (35.0/3+100)/2)
		for _, series := range r.Current.TargetTrends {
			if series.Target == "a" {
				if len(series.Points) != analytics.TrendBuckets {
					t.Fatal("unbounded trend")
				}
				found := false
				for _, p := range series.Points {
					if p.Jitter != nil {
						found = true
					}
				}
				if !found {
					t.Fatal("missing historical jitter")
				}
			}
		}
	})
}

func TestPeriodBoundaries(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for _, offset := range []time.Duration{-10*time.Minute - time.Nanosecond, -10 * time.Minute, -5*time.Minute - time.Nanosecond, -5 * time.Minute, 0, time.Nanosecond} {
			add(t, s, now.Add(offset), "tcp", "a", 10, true)
		}
		r, err := analytics.Build(context.Background(), s, "5m", now)
		if err != nil {
			t.Fatal(err)
		}
		if r.Current.Groups[0].Attempts != 2 || r.Previous.Groups[0].Attempts != 2 {
			t.Fatalf("boundary duplicated or omitted: current=%d previous=%d", r.Current.Groups[0].Attempts, r.Previous.Groups[0].Attempts)
		}
	})
}

func TestAvailabilityWindowsAndObservedStreaks(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for _, event := range []struct {
			age     time.Duration
			success bool
		}{{30 * 24 * time.Hour, true}, {7 * 24 * time.Hour, false}, {24 * time.Hour, true}, {time.Hour, false}, {0, true}} {
			add(t, s, now.Add(-event.age), "tcp", "a", 10, event.success)
		}
		add(t, s, now.Add(-31*24*time.Hour), "tcp", "too-old", 10, true)
		add(t, s, now.Add(time.Second), "tcp", "future", 10, true)
		r, err := analytics.Availability(context.Background(), s, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Targets) != 1 {
			t.Fatalf("out-of-window targets included: %+v", r.Targets)
		}
		for i, want := range []float64{50, 200.0 / 3, 50, 60} {
			near(t, r.Targets[0].Windows[i].Availability, want)
			if r.Targets[0].Windows[i].Attempts != i+2 {
				t.Fatal("missing time invented observations")
			}
		}
		target := r.Targets[0]
		if target.ConsecutiveSuccesses != 1 || target.ConsecutiveFailures != 0 || !target.LastSuccess.Equal(now) || !target.LastFailure.Equal(now.Add(-time.Hour)) {
			t.Fatalf("streak or outcome times: %+v", target)
		}
		for _, w := range r.Groups[0].Windows {
			attempts := 0
			for _, p := range w.Trend {
				attempts += p.Attempts
				if p.Attempts == 0 && p.Availability != nil {
					t.Fatal("unmeasured bucket became available")
				}
			}
			if attempts != w.Attempts {
				t.Fatal("trend totals disagree with window")
			}
		}
	})
}

func TestAnalyticsUsesMoreThanOneHistoryPage(t *testing.T) {
	stores(t, func(t *testing.T, s store.Store) {
		for i := 0; i < 1002; i++ {
			add(t, s, now.Add(-time.Duration(i)*time.Second), "tcp", "a", float64(i), true)
		}
		r, err := analytics.Build(context.Background(), s, "1h", now)
		if err != nil {
			t.Fatal(err)
		}
		if r.Current.Groups[0].Attempts != 1002 {
			t.Fatal("analytics truncated to the browser or REST page limit")
		}
		near(t, r.Current.Groups[0].P50, 500)
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := analytics.Build(cancelled, s, "5m", now); err == nil {
			t.Fatal("cancelled request succeeded")
		}
	})
}
