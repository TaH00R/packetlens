package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestResolverSummariesFailureExclusionAndRanges(t *testing.T) {
	s := store.NewMemoryStore(20)
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		stamp := at.Add(time.Duration(i) * time.Minute)
		snapshot := monitor.Snapshot{Timestamp: stamp}
		for _, endpoint := range []string{"127.0.0.1:53", "127.0.0.2:53"} {
			success := i%2 == 0
			outcome := "success"
			if !success {
				outcome = "timeout"
			}
			if endpoint == "127.0.0.2:53" {
				success = false
				outcome = "dns_error"
			}
			snapshot.DNSComparisons = append(snapshot.DNSComparisons, monitor.DNSComparison{ProbeResult: monitor.ProbeResult{Type: "dns", Target: "example.test", Timestamp: stamp, Success: success, LatencyMs: float64((i + 1) * 10), Error: map[bool]string{true: "", false: "test failure"}[success]}, Resolver: endpoint, Outcome: outcome})
		}
		if _, err := s.SaveSnapshot(context.Background(), snapshot); err != nil {
			t.Fatal(err)
		}
	}
	r, err := BuildResolvers(context.Background(), s, "5m", at.Add(3*time.Minute), []string{"127.0.0.1:53"})
	if err != nil || len(r.Groups) != 2 || len(r.Targets) != 2 {
		t.Fatal(r, err)
	}
	a, b := r.Groups[0], r.Groups[1]
	if a.Attempts != 4 || a.Successful != 2 || a.Timeouts != 2 || a.Average == nil || *a.Average != 20 || *a.Minimum != 10 || *a.Maximum != 30 || b.Average != nil || b.Timeouts != 0 || b.Failed != 4 {
		t.Fatal("incorrect successful statistics", a, b)
	}
	attempts, successes := 0, 0
	for _, p := range a.Trend {
		attempts += p.Attempts
		successes += p.Successful
		if p.Successful == 0 && p.Average != nil {
			t.Fatal("failure shown as latency")
		}
	}
	if attempts != 4 || successes != 2 {
		t.Fatal("trend dropped samples")
	}
	r, err = BuildResolvers(context.Background(), s, "5m", at.Add(time.Hour), nil)
	if err != nil || len(r.Groups) != 0 {
		t.Fatal("empty window", err)
	}
	r, err = BuildResolvers(context.Background(), s, "all", at.Add(time.Hour), nil)
	if err != nil || len(r.Groups) != 2 || !r.From.Equal(at) {
		t.Fatal("all omitted retained data", err)
	}
}
