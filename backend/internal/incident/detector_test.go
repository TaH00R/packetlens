package incident

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

var base = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func observe(s State, cfg Config, index int, at time.Time, success bool) (State, *Event) {
	return Process(s, cfg, fmt.Sprintf("%032x", index+1), 0, at, monitor.ProbeResult{Type: "tcp", Target: "a:443", Timestamp: at, Success: success, LatencyMs: 100, Error: "connection refused"})
}
func step(s State, index int, success bool) (State, *Event) {
	return observe(s, DefaultConfig(), index, base.Add(time.Duration(index)*time.Second), success)
}

func TestIsolatedFailuresAndThreshold(t *testing.T) {
	s := State{}
	for i, success := range []bool{false, true, false, false, true} {
		var e *Event
		s, e = step(s, i, success)
		if e != nil {
			t.Fatal("isolated failures became an incident")
		}
	}
	for i := 5; i < 8; i++ {
		var e *Event
		s, e = step(s, i, false)
		if i < 7 && e != nil {
			t.Fatal("incident opened below threshold")
		}
		if i == 7 && (e == nil || e.FailedProbes != 3 || !e.StartedAt.Equal(base.Add(5*time.Second)) || !e.DetectedAt.Equal(base.Add(7*time.Second))) {
			t.Fatalf("threshold/start mismatch: %+v", e)
		}
	}
	if s.Candidate != nil || s.Active == nil {
		t.Fatal("candidate did not become active")
	}
}

func TestDeduplicationInterruptedRecoveryAndPurity(t *testing.T) {
	s := State{}
	for i := 0; i < 3; i++ {
		s, _ = step(s, i, false)
	}
	before := *s.Active
	next, e := step(s, 3, false)
	if e.ID != before.ID || e.FailedProbes != 4 || *s.Active != before {
		t.Fatal("ongoing failure duplicated or mutated caller state")
	}
	next, _ = step(next, 4, true)
	if next.Active == nil || next.Active.RecoverySuccesses != 1 {
		t.Fatal("recovered before confirmation")
	}
	next, _ = step(next, 5, false)
	if next.Active.RecoverySuccesses != 0 {
		t.Fatal("failure did not reset recovery")
	}
	next, _ = step(next, 6, true)
	next, e = step(next, 7, true)
	if next.Active != nil || e.RecoveredAt == nil || e.DurationMs == nil || *e.DurationMs != 7000 || e.FailedProbes != 5 {
		t.Fatalf("recovery/duration mismatch: %+v", e)
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	_, e = step(next, 8, false)
	if e != nil {
		t.Fatal("a new isolated failure reused the recovered incident")
	}
}

func TestGapResetsCandidatesAndRecovery(t *testing.T) {
	cfg := DefaultConfig()
	s := State{}
	s, _ = step(s, 0, false)
	s, _ = step(s, 1, false)
	boundary, event := observe(s, cfg, 2, base.Add(time.Second+cfg.MaxGap), false)
	if event == nil || boundary.Active == nil || event.HasMonitoringGaps {
		t.Fatal("an exact maximum-gap boundary reset a valid failure run")
	}
	at := base.Add(time.Second + cfg.MaxGap + time.Nanosecond)
	s, e := observe(s, cfg, 2, at, false)
	if e != nil || s.Candidate == nil || s.Candidate.Failures != 1 || !s.Candidate.StartedAt.Equal(at) {
		t.Fatal("a gap counted as consecutive failures")
	}
	s, _ = observe(s, cfg, 3, at.Add(time.Second), false)
	s, _ = observe(s, cfg, 4, at.Add(2*time.Second), false)
	s, _ = observe(s, cfg, 5, at.Add(3*time.Second), true)
	later := at.Add(3*time.Second + cfg.MaxGap + time.Second)
	s, e = observe(s, cfg, 6, later, true)
	if s.Active == nil || e.RecoveredAt != nil || e.RecoverySuccesses != 1 || !e.HasMonitoringGaps {
		t.Fatal("a gap established recovery")
	}
	_, e = observe(s, cfg, 7, later.Add(time.Second), true)
	if e.RecoveredAt == nil || !e.HasMonitoringGaps {
		t.Fatal("gap metadata lost at recovery")
	}
}

func TestOutOfOrderAndFrozenPolicy(t *testing.T) {
	s := State{}
	s, _ = step(s, 0, false)
	s, _ = step(s, 1, false)
	before := s
	late, e := step(s, 0, false)
	if e != nil || !reflect.DeepEqual(late, before) {
		t.Fatal("late observation advanced state")
	}
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	cfg.RecoveryThreshold = 1
	s, e = observe(s, cfg, 2, base.Add(2*time.Second), false)
	if e == nil || e.Policy.FailureThreshold != 3 {
		t.Fatal("policy changed inside an incomplete failure run")
	}
	s, e = observe(s, cfg, 3, base.Add(3*time.Second), true)
	if s.Active == nil || e.RecoveredAt != nil {
		t.Fatal("policy changed inside an open incident")
	}
	s, _ = observe(s, cfg, 4, base.Add(4*time.Second), true)
	_, e = observe(s, cfg, 5, base.Add(5*time.Second), false)
	if e == nil || e.Policy.FailureThreshold != 1 {
		t.Fatal("new run ignored updated policy")
	}
}

func TestSingleObservationThresholdsAndZeroDuration(t *testing.T) {
	cfg := Config{FailureThreshold: 1, RecoveryThreshold: 1, MaxGap: time.Second}
	s, e := observe(State{}, cfg, 0, base, false)
	if e == nil {
		t.Fatal("threshold one did not open")
	}
	_, e = observe(s, cfg, 1, base, true)
	if e.RecoveredAt == nil || e.DurationMs == nil || *e.DurationMs != 0 {
		t.Fatal("zero-duration recovery was treated as absent")
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []Config{{0, 2, time.Second}, {1001, 2, time.Second}, {3, 0, time.Second}, {3, 101, time.Second}, {3, 2, 0}, {3, 2, 25 * time.Hour}} {
		if cfg.Validate() == nil {
			t.Fatalf("invalid policy accepted %+v", cfg)
		}
	}
}
