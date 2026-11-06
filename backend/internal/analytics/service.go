package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/store"
)

var Ranges = map[string]time.Duration{"5m": 5 * time.Minute, "15m": 15 * time.Minute, "1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour}

type Reader interface {
	HistoryBounds(context.Context) (*time.Time, *time.Time, error)
	WalkMeasurements(context.Context, store.Filter, bool, func(store.Measurement) error) error
}

func Build(ctx context.Context, source Reader, selected string, to time.Time) (Report, error) {
	r := Report{Range: selected, AsOf: to}
	duration, valid := Ranges[selected]
	if !valid && selected != "all" {
		return r, fmt.Errorf("%w: unsupported range", store.ErrInvalidQuery)
	}
	first, _, err := source.HistoryBounds(ctx)
	if err != nil {
		return r, err
	}
	r.HistoryStart = first
	from := to.Add(-duration)
	if selected == "all" {
		from = to.Add(-5 * time.Minute)
		if first != nil && first.Before(to) {
			from = *first
		}
	}
	r.Current, err = summarize(ctx, source, from, to, true)
	if err != nil {
		return r, err
	}
	if selected != "all" {
		previous, err := summarize(ctx, source, from.Add(-duration), from.Add(-time.Nanosecond), false)
		if err != nil {
			return r, err
		}
		r.Previous = &previous
	}
	return r, nil
}

func summarize(ctx context.Context, source Reader, from, to time.Time, includeTrends bool) (Period, error) {
	result := Period{From: from, To: to, Groups: []Summary{}, Targets: []Summary{}, Trends: []Series{}, TargetTrends: []Series{}}
	groups := map[string]*counter{"tcp": newCounter("tcp", ""), "dns": newCounter("dns", "")}
	targets := map[string]*counter{}
	groupTrends := map[string]*timeline{"tcp": newTimeline(from, to), "dns": newTimeline(from, to)}
	targetTrends := map[string]*timeline{}
	err := source.WalkMeasurements(ctx, store.Filter{From: &from, To: &to}, true, func(m store.Measurement) error {
		key := m.Probe.Type + ":" + m.Probe.Target
		target := targets[key]
		if target == nil {
			if len(targets) >= MaxTargets {
				return fmt.Errorf("analytics target limit (%d) exceeded", MaxTargets)
			}
			target = newCounter(m.Probe.Type, m.Probe.Target)
			targets[key] = target
			if includeTrends {
				targetTrends[key] = newTimeline(from, to)
			}
		}
		difference := target.observe(m, m.TargetRank, m.TargetCount, true)
		groups[m.Probe.Type].observe(m, m.TypeRank, m.TypeCount, false)
		if includeTrends {
			groupTrends[m.Probe.Type].observe(m, difference)
			targetTrends[key].observe(m, difference)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		c := targets[key]
		result.Targets = append(result.Targets, c.finish())
		if includeTrends {
			result.TargetTrends = append(result.TargetTrends, Series{Type: c.Type, Target: c.Target, Points: targetTrends[key].points()})
		}
	}
	for _, typ := range []string{"tcp", "dns"} {
		g := groups[typ].finish()
		var sum float64
		n := 0
		for _, t := range result.Targets {
			if t.Type == typ && t.Jitter != nil {
				sum += *t.Jitter
				n++
			}
		}
		if n > 0 {
			g.Jitter = number(sum / float64(n))
		}
		result.Groups = append(result.Groups, g)
		if includeTrends {
			result.Trends = append(result.Trends, Series{Type: typ, Points: groupTrends[typ].points()})
		}
	}
	return result, nil
}
