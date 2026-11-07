package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

type ResolverSummary struct {
	Resolver      string     `json:"resolver"`
	Target        string     `json:"target"`
	Attempts      int        `json:"attempts"`
	Successful    int        `json:"successful"`
	Failed        int        `json:"failed"`
	Timeouts      int        `json:"timeouts"`
	Average       *float64   `json:"avg_latency_ms"`
	Minimum       *float64   `json:"min_latency_ms"`
	Maximum       *float64   `json:"max_latency_ms"`
	LastFailureAt *time.Time `json:"last_failure_at"`
	LastError     string     `json:"last_error"`
	Trend         []Trend    `json:"trend"`
}
type ResolverReport struct {
	Range      string            `json:"range"`
	AsOf       time.Time         `json:"as_of"`
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	Configured []string          `json:"configured_resolvers"`
	Groups     []ResolverSummary `json:"groups"`
	Targets    []ResolverSummary `json:"targets"`
}
type ResolverReader interface {
	HistoryBounds(context.Context) (*time.Time, *time.Time, error)
	WalkDNSComparisons(context.Context, store.Filter, func(time.Time, monitor.DNSComparison) error) error
}
type resolverCounter struct {
	summary  ResolverSummary
	timeline *timeline
}

func BuildResolvers(ctx context.Context, source ResolverReader, selected string, to time.Time, configured []string) (ResolverReport, error) {
	r := ResolverReport{Range: selected, AsOf: to, To: to, Configured: append([]string{}, configured...), Groups: []ResolverSummary{}, Targets: []ResolverSummary{}}
	duration, ok := Ranges[selected]
	if !ok && selected != "all" {
		return r, fmt.Errorf("%w: unsupported range", store.ErrInvalidQuery)
	}
	r.From = to.Add(-duration)
	if selected == "all" {
		first, _, err := source.HistoryBounds(ctx)
		if err != nil {
			return r, err
		}
		r.From = to.Add(-5 * time.Minute)
		if first != nil && first.Before(to) {
			r.From = *first
		}
	}
	groups := map[string]*resolverCounter{}
	targets := map[string]*resolverCounter{}
	observe := func(c *resolverCounter, at time.Time, p monitor.DNSComparison) {
		s := &c.summary
		s.Attempts++
		if p.Success {
			s.Successful++
			if s.Average == nil {
				s.Average = number(p.LatencyMs)
			} else {
				*s.Average += (p.LatencyMs - *s.Average) / float64(s.Successful)
			}
			if s.Minimum == nil || p.LatencyMs < *s.Minimum {
				s.Minimum = number(p.LatencyMs)
			}
			if s.Maximum == nil || p.LatencyMs > *s.Maximum {
				s.Maximum = number(p.LatencyMs)
			}
		} else {
			s.Failed++
			if p.Outcome == "timeout" {
				s.Timeouts++
			}
			if s.LastFailureAt == nil || !p.Timestamp.Before(*s.LastFailureAt) {
				stamp := p.Timestamp
				s.LastFailureAt = &stamp
				s.LastError = p.Error
			}
		}
		c.timeline.observe(store.Measurement{SnapshotTimestamp: at, Probe: p.ProbeResult}, nil)
	}
	err := source.WalkDNSComparisons(ctx, store.Filter{From: &r.From, To: &to, Type: "dns"}, func(at time.Time, p monitor.DNSComparison) error {
		key := p.Resolver + "|" + p.Target
		if targets[key] == nil {
			if len(targets) >= MaxTargets {
				return fmt.Errorf("resolver analytics identity limit (%d) exceeded", MaxTargets)
			}
			targets[key] = &resolverCounter{summary: ResolverSummary{Resolver: p.Resolver, Target: p.Target}, timeline: newTimeline(r.From, to)}
		}
		if groups[p.Resolver] == nil {
			groups[p.Resolver] = &resolverCounter{summary: ResolverSummary{Resolver: p.Resolver}, timeline: newTimeline(r.From, to)}
		}
		observe(targets[key], at, p)
		observe(groups[p.Resolver], at, p)
		return nil
	})
	if err != nil {
		return r, err
	}
	finish := func(values map[string]*resolverCounter) []ResolverSummary {
		keys := []string{}
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := []ResolverSummary{}
		for _, key := range keys {
			c := values[key]
			c.summary.Trend = c.timeline.points()
			out = append(out, c.summary)
		}
		return out
	}
	r.Groups = finish(groups)
	r.Targets = finish(targets)
	return r, nil
}
