package correlation

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

var base = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func cycle(second int, outcomes ...Observation) monitor.Snapshot {
	s := monitor.Snapshot{ID: fmt.Sprintf("%032x", second+1), Timestamp: base.Add(time.Duration(second) * time.Second)}
	for _, o := range outcomes {
		p := monitor.ProbeResult{Type: o.Type, Target: o.Target, Success: o.Success, Timestamp: s.Timestamp}
		if p.Type == "tcp" {
			s.TCP = append(s.TCP, p)
		} else {
			s.DNS = append(s.DNS, p)
		}
	}
	return s
}
func TestOverlapLifecycleAndCloning(t *testing.T) {
	cfg := DefaultConfig()
	a := Observation{Type: "tcp", Target: "a"}
	b := Observation{Type: "dns", Target: "b"}
	state, events, err := Process(State{}, cfg, cycle(0, a))
	if err != nil || len(events) != 0 {
		t.Fatal("single failure correlated", err)
	}
	state, events, err = Process(state, cfg, cycle(15, b))
	if err != nil || len(events) != 1 || len(events[0].Members) != 2 {
		t.Fatal("inclusive boundary missed", err)
	}
	id := events[0].ID
	saved := state
	state, events, err = Process(state, cfg, cycle(16, a))
	if err != nil || len(events) != 1 || events[0].ID != id || !saved.Active.LastCorrelatedAt.Equal(base.Add(15*time.Second)) {
		t.Fatal("duplicate or mutated state", err)
	}
	b.Success = true
	state, events, err = Process(state, cfg, cycle(17, b))
	if err != nil || len(events) != 1 || events[0].EndedAt == nil || state.Active != nil {
		t.Fatal("success did not end overlap", err)
	}
	b.Success = false
	state, events, err = Process(state, cfg, cycle(18, b))
	if err != nil || len(events) != 1 || events[0].ID == id {
		t.Fatal("new episode missing", err)
	}
	old := state
	next, events, err := Process(state, cfg, cycle(5, a))
	if err != nil || len(events) != 0 || !reflect.DeepEqual(old, next) {
		t.Fatal("late cycle changed state", err)
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestExpiryDistinctNamesAndGap(t *testing.T) {
	cfg := DefaultConfig()
	a := Observation{Type: "tcp", Target: "a"}
	same := Observation{Type: "dns", Target: "a"}
	b := Observation{Type: "dns", Target: "b"}
	s, e, _ := Process(State{}, cfg, cycle(0, a, same))
	if len(e) != 0 {
		t.Fatal("same target name counted twice")
	}
	s, e, _ = Process(s, cfg, cycle(16, b))
	if len(e) != 0 {
		t.Fatal("expired failure correlated")
	}
	s, e, _ = Process(s, cfg, cycle(17, a))
	if len(e) != 1 {
		t.Fatal("overlap missing")
	}
	id := e[0].ID
	s, e, _ = Process(s, cfg, cycle(40, a, b))
	if len(e) != 2 || e[0].EndedAt == nil || e[1].ID == id || s.Active == nil {
		t.Fatal("monitoring gap bridged episodes")
	}
	s, e, _ = Process(s, cfg, cycle(56))
	if len(e) != 1 || e[0].EndedAt == nil || s.Active != nil {
		t.Fatal("expiry did not end episode")
	}
}
func TestFrozenPolicyLimitsAndOrdering(t *testing.T) {
	cfg := DefaultConfig()
	a := Observation{Type: "tcp", Target: "a"}
	b := Observation{Type: "dns", Target: "b"}
	s, _, _ := Process(State{}, cfg, cycle(0, a, b))
	changed := Config{Window: time.Second, MinTargets: 3}
	s, e, err := Process(s, changed, cycle(5, a, b))
	if err != nil || len(e) != 1 || e[0].Policy != cfg {
		t.Fatal("open policy changed", err)
	}
	late := cycle(6)
	late.TCP = []monitor.ProbeResult{{Type: "tcp", Target: "a", Success: true, Timestamp: base.Add(-time.Second)}}
	s, e, err = Process(s, changed, late)
	if err != nil || len(e) != 1 || s.Active == nil {
		t.Fatal("older success superseded newer failure", err)
	}
	tooMany := cycle(7)
	for i := 0; i <= MaxTargets; i++ {
		tooMany.TCP = append(tooMany.TCP, monitor.ProbeResult{Type: "tcp", Target: fmt.Sprint(i), Timestamp: tooMany.Timestamp})
	}
	next, _, err := Process(s, cfg, tooMany)
	if err == nil || !reflect.DeepEqual(s, next) {
		t.Fatal("unbounded or partially advanced state")
	}
	for _, bad := range []Config{{Window: 0, MinTargets: 2}, {Window: time.Hour + 1, MinTargets: 2}, {Window: time.Second, MinTargets: 1}, {Window: time.Second, MinTargets: 129}} {
		if bad.Validate() == nil {
			t.Fatal("accepted invalid config")
		}
	}
}
