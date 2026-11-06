// Package incident tracks sustained observed probe failures without inferring
// outcomes during monitoring gaps or attributing a root cause.
package incident

import (
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type Config struct {
	FailureThreshold  int           `json:"failure_threshold"`
	RecoveryThreshold int           `json:"recovery_threshold"`
	MaxGap            time.Duration `json:"max_gap_ns"`
}

func DefaultConfig() Config {
	return Config{FailureThreshold: 3, RecoveryThreshold: 2, MaxGap: 30 * time.Second}
}
func (c Config) Validate() error {
	if c.FailureThreshold < 1 || c.FailureThreshold > 1000 {
		return fmt.Errorf("incident failure threshold must be 1..1000")
	}
	if c.RecoveryThreshold < 1 || c.RecoveryThreshold > 100 {
		return fmt.Errorf("incident recovery threshold must be 1..100")
	}
	if c.MaxGap < time.Second || c.MaxGap > 24*time.Hour {
		return fmt.Errorf("incident maximum observation gap must be 1s..24h")
	}
	return nil
}

type Event struct {
	ID                string     `json:"id"`
	Type              string     `json:"type"`
	Target            string     `json:"target"`
	StartedAt         time.Time  `json:"started_at"`
	CycleTimestamp    time.Time  `json:"cycle_timestamp"`
	DetectedAt        time.Time  `json:"detected_at"`
	DetectedCycle     time.Time  `json:"detected_cycle"`
	LastFailureAt     time.Time  `json:"last_failure_at"`
	LastObservedAt    time.Time  `json:"last_observed_at"`
	RecoveredAt       *time.Time `json:"recovered_at"`
	RecoveryCycle     *time.Time `json:"recovery_cycle"`
	DurationMs        *float64   `json:"duration_ms"`
	FailedProbes      int        `json:"failed_probes"`
	RecoverySuccesses int        `json:"recovery_successes"`
	HasMonitoringGaps bool       `json:"has_monitoring_gaps"`
	FirstError        string     `json:"first_error"`
	LastError         string     `json:"last_error"`
	Policy            Config     `json:"policy"`
}

type Candidate struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Target         string    `json:"target"`
	StartedAt      time.Time `json:"started_at"`
	CycleTimestamp time.Time `json:"cycle_timestamp"`
	Failures       int       `json:"failures"`
	FirstError     string    `json:"first_error"`
	Policy         Config    `json:"policy"`
}

type State struct {
	LastObserved time.Time  `json:"last_observed"`
	Candidate    *Candidate `json:"candidate"`
	Active       *Event     `json:"active"`
}

func boundedError(value string) string {
	runes := []rune(value)
	if len(runes) > 1024 {
		runes = runes[:1024]
	}
	return string(runes)
}

// Process returns cloned state. A candidate is not an incident record until its
// threshold is reached; storage commits these transitions with the raw cycle.
func Process(previous State, cfg Config, snapshotID string, ordinal int, cycle time.Time, p monitor.ProbeResult) (State, *Event) {
	if !previous.LastObserved.IsZero() && p.Timestamp.Before(previous.LastObserved) {
		return previous, nil
	}
	s := previous
	if s.Candidate != nil {
		copy := *s.Candidate
		s.Candidate = &copy
	}
	if s.Active != nil {
		copy := *s.Active
		s.Active = &copy
	}
	policy := cfg
	if s.Active != nil {
		policy = s.Active.Policy
	} else if s.Candidate != nil {
		policy = s.Candidate.Policy
	}
	if !s.LastObserved.IsZero() && p.Timestamp.Sub(s.LastObserved) > policy.MaxGap {
		s.Candidate = nil
		if s.Active != nil {
			s.Active.RecoverySuccesses = 0
			s.Active.HasMonitoringGaps = true
		}
	}
	s.LastObserved = p.Timestamp
	if s.Active != nil {
		e := s.Active
		e.LastObservedAt = p.Timestamp
		if !p.Success {
			e.FailedProbes++
			e.RecoverySuccesses = 0
			e.LastFailureAt = p.Timestamp
			e.LastError = boundedError(p.Error)
			return s, e
		}
		e.RecoverySuccesses++
		if e.RecoverySuccesses >= e.Policy.RecoveryThreshold {
			recovered, recoveryCycle := p.Timestamp, cycle
			duration := p.Timestamp.Sub(e.StartedAt).Seconds() * 1000
			e.RecoveredAt = &recovered
			e.RecoveryCycle = &recoveryCycle
			e.DurationMs = &duration
			s.Active = nil
		}
		return s, e
	}
	if p.Success {
		s.Candidate = nil
		return s, nil
	}
	if s.Candidate == nil {
		s.Candidate = &Candidate{ID: snapshotID + "-" + strconv.Itoa(ordinal), Type: p.Type, Target: p.Target, StartedAt: p.Timestamp, CycleTimestamp: cycle, FirstError: boundedError(p.Error), Policy: cfg}
	}
	c := s.Candidate
	c.Failures++
	if c.Failures < c.Policy.FailureThreshold {
		return s, nil
	}
	e := &Event{ID: c.ID, Type: p.Type, Target: p.Target, StartedAt: c.StartedAt, CycleTimestamp: c.CycleTimestamp, DetectedAt: p.Timestamp, DetectedCycle: cycle,
		LastFailureAt: p.Timestamp, LastObservedAt: p.Timestamp, FailedProbes: c.Failures, FirstError: c.FirstError, LastError: boundedError(p.Error), Policy: c.Policy}
	s.Candidate = nil
	s.Active = e
	return s, e
}

func (e Event) Validate() error {
	if err := e.Policy.Validate(); err != nil {
		return err
	}
	if (e.Type != "tcp" && e.Type != "dns") || e.Target == "" || len(e.Target) > 512 || e.StartedAt.IsZero() || e.CycleTimestamp.IsZero() || e.DetectedCycle.IsZero() || e.DetectedAt.Before(e.StartedAt) || e.LastFailureAt.Before(e.DetectedAt) || e.LastObservedAt.Before(e.LastFailureAt) {
		return fmt.Errorf("invalid incident identity or timestamps")
	}
	if e.FailedProbes < e.Policy.FailureThreshold || e.RecoverySuccesses < 0 || e.RecoverySuccesses > e.Policy.RecoveryThreshold || utf8.RuneCountInString(e.FirstError) > 1024 || utf8.RuneCountInString(e.LastError) > 1024 {
		return fmt.Errorf("invalid incident counts or errors")
	}
	if e.RecoveredAt == nil {
		if e.RecoveryCycle != nil || e.DurationMs != nil || e.RecoverySuccesses >= e.Policy.RecoveryThreshold {
			return fmt.Errorf("invalid open incident recovery")
		}
	} else {
		if e.RecoveryCycle == nil || e.DurationMs == nil || math.IsNaN(*e.DurationMs) || math.IsInf(*e.DurationMs, 0) || *e.DurationMs < 0 || !e.RecoveredAt.Equal(e.LastObservedAt) || e.RecoverySuccesses != e.Policy.RecoveryThreshold {
			return fmt.Errorf("invalid recovered incident")
		}
		if math.Abs(*e.DurationMs-e.RecoveredAt.Sub(e.StartedAt).Seconds()*1000) > .001 {
			return fmt.Errorf("incident duration does not match observations")
		}
	}
	return nil
}

func (s State) Validate() error {
	if s.LastObserved.IsZero() || (s.Candidate != nil && s.Active != nil) {
		return fmt.Errorf("invalid incident state")
	}
	if c := s.Candidate; c != nil {
		if err := c.Policy.Validate(); err != nil {
			return err
		}
		if (c.Type != "tcp" && c.Type != "dns") || c.Target == "" || len(c.Target) > 512 || c.Failures < 1 || c.Failures >= c.Policy.FailureThreshold || c.StartedAt.IsZero() || c.CycleTimestamp.IsZero() || c.StartedAt.After(s.LastObserved) || utf8.RuneCountInString(c.FirstError) > 1024 {
			return fmt.Errorf("invalid incident candidate")
		}
	}
	if e := s.Active; e != nil {
		if err := e.Validate(); err != nil {
			return err
		}
		if e.RecoveredAt != nil || !e.LastObservedAt.Equal(s.LastObserved) {
			return fmt.Errorf("invalid active incident state")
		}
	}
	return nil
}
