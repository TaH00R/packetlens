// Package correlation groups recent failed observations, without attributing a cause.
package correlation

import (
	"fmt"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

const MaxTargets = 128

type Config struct {
	Window     time.Duration `json:"window_ns"`
	MinTargets int           `json:"min_targets"`
}

func DefaultConfig() Config { return Config{Window: 15 * time.Second, MinTargets: 2} }
func (c Config) Validate() error {
	if c.Window < time.Second || c.Window > time.Hour || c.MinTargets < 2 || c.MinTargets > MaxTargets {
		return fmt.Errorf("correlation window must be 1s..1h and minimum targets 2..128")
	}
	return nil
}

type Observation struct {
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}
type Member struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}
type Event struct {
	ID               string     `json:"id"`
	CycleTimestamp   time.Time  `json:"cycle_timestamp"`
	DetectedAt       time.Time  `json:"detected_at"`
	LastCorrelatedAt time.Time  `json:"last_correlated_at"`
	EndedAt          *time.Time `json:"ended_at"`
	EndCycle         *time.Time `json:"end_cycle"`
	Members          []Member   `json:"members"`
	Policy           Config     `json:"policy"`
}
type State struct {
	LastCycle time.Time     `json:"last_cycle"`
	Watermark time.Time     `json:"watermark"`
	Recent    []Observation `json:"recent"`
	Active    *Event        `json:"active"`
}

func key(m Member) string { return m.Type + ":" + m.Target }
func validMember(m Member) bool {
	return (m.Type == "tcp" || m.Type == "dns") && m.Target != "" && len(m.Target) <= 512
}
func (e Event) Validate() error {
	if err := e.Policy.Validate(); err != nil {
		return err
	}
	if e.ID == "" || e.CycleTimestamp.IsZero() || e.DetectedAt.IsZero() || e.DetectedAt.Before(e.CycleTimestamp) || e.LastCorrelatedAt.Before(e.DetectedAt) || len(e.Members) > MaxTargets {
		return fmt.Errorf("invalid correlation event")
	}
	seen := map[string]bool{}
	targets := map[string]bool{}
	for _, m := range e.Members {
		if !validMember(m) || seen[key(m)] {
			return fmt.Errorf("invalid correlation members")
		}
		seen[key(m)] = true
		targets[m.Target] = true
	}
	if len(targets) < e.Policy.MinTargets {
		return fmt.Errorf("insufficient correlation members")
	}
	if (e.EndedAt == nil) != (e.EndCycle == nil) || (e.EndedAt != nil && (e.EndedAt.Before(e.LastCorrelatedAt) || e.EndCycle.Before(e.CycleTimestamp) || e.EndCycle.After(*e.EndedAt))) {
		return fmt.Errorf("invalid correlation end")
	}
	return nil
}
func (s State) Validate() error {
	if s.LastCycle.IsZero() || s.Watermark.Before(s.LastCycle) || len(s.Recent) > MaxTargets {
		return fmt.Errorf("invalid correlation state")
	}
	seen := map[string]bool{}
	for _, p := range s.Recent {
		m := Member{p.Type, p.Target}
		if !validMember(m) || p.Timestamp.IsZero() || p.Timestamp.After(s.Watermark) || seen[key(m)] {
			return fmt.Errorf("invalid recent correlation observation")
		}
		seen[key(m)] = true
	}
	if s.Active != nil {
		if err := s.Active.Validate(); err != nil {
			return err
		}
		if s.Active.EndedAt != nil || s.Active.LastCorrelatedAt.After(s.Watermark) || s.Active.CycleTimestamp.After(s.LastCycle) {
			return fmt.Errorf("invalid active correlation")
		}
	}
	return nil
}
func eligible(recent []Observation, at time.Time, cfg Config) []Member {
	members := []Member{}
	targets := map[string]bool{}
	for _, p := range recent {
		if !p.Success && at.Sub(p.Timestamp) <= cfg.Window {
			members = append(members, Member{p.Type, p.Target})
			targets[p.Target] = true
		}
	}
	if len(targets) < cfg.MinTargets {
		return nil
	}
	return members
}

// Process evaluates once per complete cycle. Newer successes supersede failures.
// Ending an overlap does not assert that its members recovered. Events and state
// are cloned so a failed storage transaction cannot partially advance detection.
func Process(previous State, cfg Config, snapshot monitor.Snapshot) (State, []Event, error) {
	if err := cfg.Validate(); err != nil {
		return previous, nil, err
	}
	if snapshot.Timestamp.Before(previous.LastCycle) {
		return previous, nil, nil
	}
	s := previous
	s.Recent = append([]Observation{}, previous.Recent...)
	if s.Active != nil {
		e := *s.Active
		e.Members = append([]Member{}, e.Members...)
		s.Active = &e
	}
	s.LastCycle = snapshot.Timestamp
	if snapshot.Timestamp.After(s.Watermark) {
		s.Watermark = snapshot.Timestamp
	}
	for _, group := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
		for _, p := range group {
			if p.Timestamp.After(s.Watermark) {
				s.Watermark = p.Timestamp
			}
		}
	}
	// Retain one outcome per identity for at most the maximum supported window.
	// A future larger policy can then use those recent outcomes after a restart.
	recent := map[string]Observation{}
	for _, p := range s.Recent {
		if s.Watermark.Sub(p.Timestamp) <= time.Hour {
			recent[key(Member{p.Type, p.Target})] = p
		}
	}
	for _, group := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
		for _, p := range group {
			k := key(Member{p.Type, p.Target})
			old, ok := recent[k]
			if s.Watermark.Sub(p.Timestamp) <= time.Hour && (!ok || !p.Timestamp.Before(old.Timestamp)) {
				recent[k] = Observation{p.Type, p.Target, p.Timestamp, p.Success}
			}
		}
	}
	if len(recent) > MaxTargets {
		return previous, nil, fmt.Errorf("correlation supports at most %d recent probe identities", MaxTargets)
	}
	s.Recent = []Observation{}
	for _, p := range recent {
		s.Recent = append(s.Recent, p)
	}
	sort.Slice(s.Recent, func(i, j int) bool {
		return key(Member{s.Recent[i].Type, s.Recent[i].Target}) < key(Member{s.Recent[j].Type, s.Recent[j].Target})
	})
	updates := []Event{}
	if e := s.Active; e != nil {
		members := eligible(s.Recent, s.Watermark, e.Policy)
		if len(members) == 0 || s.Watermark.Sub(e.LastCorrelatedAt) > e.Policy.Window {
			at, cycle := s.Watermark, snapshot.Timestamp
			e.EndedAt = &at
			e.EndCycle = &cycle
			updates = append(updates, *e)
			s.Active = nil
		}
	}
	policy := cfg
	if s.Active != nil {
		policy = s.Active.Policy
	}
	members := eligible(s.Recent, s.Watermark, policy)
	if len(members) > 0 {
		if s.Active == nil {
			s.Active = &Event{ID: snapshot.ID + "-0", CycleTimestamp: snapshot.Timestamp, DetectedAt: s.Watermark, LastCorrelatedAt: s.Watermark, Members: []Member{}, Policy: cfg}
		}
		e := s.Active
		union := map[string]Member{}
		for _, m := range e.Members {
			union[key(m)] = m
		}
		for _, m := range members {
			union[key(m)] = m
		}
		if len(union) > MaxTargets {
			return previous, nil, fmt.Errorf("correlation episode exceeds %d members", MaxTargets)
		}
		e.Members = []Member{}
		for _, m := range union {
			e.Members = append(e.Members, m)
		}
		sort.Slice(e.Members, func(i, j int) bool { return key(e.Members[i]) < key(e.Members[j]) })
		e.LastCorrelatedAt = s.Watermark
		updates = append(updates, *e)
	}
	return s, updates, nil
}
