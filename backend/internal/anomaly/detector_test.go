package anomaly

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

func testConfig() Config {
	c := DefaultConfig()
	c.Window = 5
	c.MinSamples = 3
	c.Multiplier = 3
	c.RecoverySamples = 2
	return c
}
func probe(at time.Time, latency float64, success bool) monitor.ProbeResult {
	return monitor.ProbeResult{Type: "tcp", Target: "a:443", Timestamp: at, LatencyMs: latency, Success: success}
}
func step(s State, c Config, i int, latency float64, success bool) (State, *Event) {
	at := time.Date(2026, 11, 7, 0, 0, i, 0, time.UTC)
	return Process(s, c, fmt.Sprintf("%032x", i+1), 0, at, probe(at, latency, success))
}

func TestWarmupAndZeroMAD(t *testing.T) {
	c := testConfig()
	s := State{}
	for i := 0; i < 3; i++ {
		var e *Event
		s, e = step(s, c, i, 10, true)
		if e != nil {
			t.Fatal("alert during warmup")
		}
	}
	s, e := step(s, c, 3, 20, true)
	if e != nil {
		t.Fatal("threshold must be strictly exceeded")
	}
	s, e = step(s, c, 4, 21, true)
	if e == nil || e.Baseline != 10 || e.MAD != 0 || e.Threshold != 20 || e.Severity != "warning" || len(s.Samples) != 4 {
		t.Fatalf("zero MAD handling: %+v", e)
	}
}

func TestMADFreezeSuppressionRecoveryAndPurity(t *testing.T) {
	c := testConfig()
	s := State{}
	for i, v := range []float64{10, 20, 30} {
		s, _ = step(s, c, i, v, true)
	}
	s, e := step(s, c, 3, 51, true)
	if e == nil || e.Baseline != 20 || e.MAD != 10 || e.Threshold != 50 {
		t.Fatalf("median/MAD: %+v", e)
	}
	original := s
	originalJSON := *original.Active
	next, e := step(s, c, 4, 100, true)
	if e.ID != originalJSON.ID || e.AnomalousSamples != 2 || e.PeakLatency != 100 || e.Severity != "critical" || e.Threshold != 50 {
		t.Fatalf("ongoing episode: %+v", e)
	}
	if *original.Active != originalJSON {
		t.Fatal("Process mutated the caller's active episode")
	}
	next, _ = step(next, c, 5, 40, true)
	if next.Active == nil || len(next.Recovery) != 1 {
		t.Fatal("recovery prematurely closed")
	}
	next, _ = step(next, c, 6, 0, false)
	if len(next.Recovery) != 0 || next.Active == nil {
		t.Fatal("failure counted as normal recovery")
	}
	next, _ = step(next, c, 7, 40, true)
	next, e = step(next, c, 8, 40, true)
	if next.Active != nil || e.RecoveredAt == nil || e.AnomalousSamples != 2 {
		t.Fatalf("recovery: %+v", e)
	}
	if len(next.Samples) > c.Window {
		t.Fatal("unbounded baseline")
	}
}

func TestExpiryOutOfOrderAndFailureExclusion(t *testing.T) {
	c := testConfig()
	s := State{}
	for i := 0; i < 10; i++ {
		s, _ = step(s, c, i, 10, true)
	}
	if len(s.Samples) != c.Window {
		t.Fatal("rolling window did not evict")
	}
	before := s
	s, e := step(s, c, 8, 100, true)
	if e != nil || !reflect.DeepEqual(before, s) {
		t.Fatal("out-of-order probe advanced the baseline")
	}
	s, _ = step(s, c, 11, 3000, false)
	if len(s.Samples) != c.Window {
		t.Fatal("failure entered baseline")
	}
	at := s.LastObserved.Add(c.MaxAge + time.Second)
	s, e = Process(s, c, fmt.Sprintf("%032x", 999), 0, at, probe(at, 100, true))
	if e != nil || len(s.Samples) != 1 {
		t.Fatal("expired baseline generated an anomaly")
	}
}

func TestConfigValidation(t *testing.T) {
	for _, mutate := range []func(*Config){func(c *Config) { c.Window = 501 }, func(c *Config) { c.MinSamples = 2 }, func(c *Config) { c.Multiplier = 0 }, func(c *Config) { c.MinDeltaMs = 0 }, func(c *Config) { c.MaxAge = 0 }, func(c *Config) { c.RecoverySamples = 0 }} {
		c := DefaultConfig()
		mutate(&c)
		if c.Validate() == nil {
			t.Fatalf("invalid config accepted: %+v", c)
		}
	}
}

func TestGapDoesNotEstablishRecovery(t *testing.T) {
	c := testConfig()
	s := State{}
	for i := 0; i < 3; i++ {
		s, _ = step(s, c, i, 10, true)
	}
	s, _ = step(s, c, 3, 100, true)
	s, _ = step(s, c, 4, 10, true)
	at := s.LastObserved.Add(c.MaxAge + time.Minute)
	s, e := Process(s, c, fmt.Sprintf("%032x", 100), 0, at, probe(at, 10, true))
	if s.Active == nil || e.RecoveredAt != nil || len(s.Recovery) != 1 {
		t.Fatal("a monitoring gap counted toward recovery")
	}
}
