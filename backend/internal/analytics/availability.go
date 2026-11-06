package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/store"
)

var windows = []struct {
	name     string
	duration time.Duration
}{{"1h", time.Hour}, {"24h", 24 * time.Hour}, {"7d", 7 * 24 * time.Hour}, {"30d", 30 * 24 * time.Hour}}

type availabilityCounter struct {
	AvailabilityTarget
	trends []*timeline
}

func newAvailability(typ, target string, to time.Time, trends bool) *availabilityCounter {
	c := &availabilityCounter{AvailabilityTarget: AvailabilityTarget{Type: typ, Target: target, Windows: []AvailabilityWindow{}}}
	for _, w := range windows {
		c.Windows = append(c.Windows, AvailabilityWindow{Window: w.name, From: to.Add(-w.duration), To: to, Trend: []Trend{}})
		if trends {
			c.trends = append(c.trends, newTimeline(to.Add(-w.duration), to))
		}
	}
	return c
}

func (c *availabilityCounter) observe(m store.Measurement, streak bool) {
	if streak {
		t := m.Probe.Timestamp
		if m.Probe.Success {
			c.ConsecutiveSuccesses++
			c.ConsecutiveFailures = 0
			c.LastSuccess = &t
		} else {
			c.ConsecutiveFailures++
			c.ConsecutiveSuccesses = 0
			c.LastFailure = &t
		}
	}
	for i := range c.Windows {
		w := &c.Windows[i]
		if m.SnapshotTimestamp.Before(w.From) || m.SnapshotTimestamp.After(w.To) {
			continue
		}
		w.Attempts++
		if m.Probe.Success {
			w.Successful++
		}
		if w.FirstObserved == nil || m.SnapshotTimestamp.Before(*w.FirstObserved) {
			t := m.SnapshotTimestamp
			w.FirstObserved = &t
		}
		if w.LastObserved == nil || m.SnapshotTimestamp.After(*w.LastObserved) {
			t := m.SnapshotTimestamp
			w.LastObserved = &t
		}
		if len(c.trends) > 0 {
			c.trends[i].observe(m, nil)
		}
	}
}

func (c *availabilityCounter) finish() AvailabilityTarget {
	for i := range c.Windows {
		c.Windows[i].Availability = rate(c.Windows[i].Successful, c.Windows[i].Attempts)
		if len(c.trends) > 0 {
			c.Windows[i].Trend = c.trends[i].points()
		}
	}
	return c.AvailabilityTarget
}

func Availability(ctx context.Context, source Reader, to time.Time) (AvailabilityReport, error) {
	r := AvailabilityReport{AsOf: to, Groups: []AvailabilityTarget{}, Targets: []AvailabilityTarget{}}
	first, _, err := source.HistoryBounds(ctx)
	if err != nil {
		return r, err
	}
	r.HistoryStart = first
	from := to.Add(-30 * 24 * time.Hour)
	groups := map[string]*availabilityCounter{"tcp": newAvailability("tcp", "", to, true), "dns": newAvailability("dns", "", to, true)}
	targets := map[string]*availabilityCounter{}
	err = source.WalkMeasurements(ctx, store.Filter{From: &from, To: &to}, false, func(m store.Measurement) error {
		key := m.Probe.Type + ":" + m.Probe.Target
		t := targets[key]
		if t == nil {
			if len(targets) >= MaxTargets {
				return fmt.Errorf("availability target limit (%d) exceeded", MaxTargets)
			}
			t = newAvailability(m.Probe.Type, m.Probe.Target, to, false)
			targets[key] = t
		}
		t.observe(m, true)
		groups[m.Probe.Type].observe(m, false)
		return nil
	})
	if err != nil {
		return r, err
	}
	for _, typ := range []string{"tcp", "dns"} {
		r.Groups = append(r.Groups, groups[typ].finish())
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r.Targets = append(r.Targets, targets[key].finish())
	}
	return r, nil
}
