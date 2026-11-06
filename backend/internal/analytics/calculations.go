package analytics

import (
	"math"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func number(value float64) *float64 { return &value }

// NearestRank is one-based: ceil(percentile/100 * successful count).
// No interpolation is performed; an empty sample has no rank.
func NearestRank(count, percentile int) int {
	if count < 1 || percentile < 1 || percentile > 100 {
		return 0
	}
	return int(math.Ceil(float64(count) * float64(percentile) / 100))
}

func rate(success, total int) *float64 {
	if total == 0 {
		return nil
	}
	return number(100 * float64(success) / float64(total))
}

func histogram() []Bucket {
	edges := []float64{0, 10, 25, 50, 100, 250, 500, 1000}
	bins := make([]Bucket, len(edges))
	for i, lower := range edges {
		bins[i].Lower = lower
		if i+1 < len(edges) {
			bins[i].Upper = number(edges[i+1])
		}
	}
	return bins
}

type counter struct {
	Summary
	previous        *float64
	pairs           int
	differenceTotal float64
}

func newCounter(typ, target string) *counter {
	return &counter{Summary: Summary{Type: typ, Target: target, Histogram: histogram()}}
}

func (c *counter) observe(m store.Measurement, rank, count int, compare bool) *float64 {
	p := m.Probe
	c.Attempts++
	if c.FirstObserved == nil || m.SnapshotTimestamp.Before(*c.FirstObserved) {
		t := m.SnapshotTimestamp
		c.FirstObserved = &t
	}
	if c.LastObserved == nil || m.SnapshotTimestamp.After(*c.LastObserved) {
		t := m.SnapshotTimestamp
		c.LastObserved = &t
	}
	if !p.Success {
		c.Failed++
		return nil
	}
	c.Successful++
	if c.Average == nil {
		c.Average = number(p.LatencyMs)
	} else {
		*c.Average += (p.LatencyMs - *c.Average) / float64(c.Successful)
	}
	if c.Minimum == nil || p.LatencyMs < *c.Minimum {
		c.Minimum = number(p.LatencyMs)
	}
	if c.Maximum == nil || p.LatencyMs > *c.Maximum {
		c.Maximum = number(p.LatencyMs)
	}
	for _, q := range []struct {
		percentile  int
		destination **float64
	}{{50, &c.P50}, {90, &c.P90}, {95, &c.P95}, {99, &c.P99}} {
		if rank == NearestRank(count, q.percentile) {
			*q.destination = number(p.LatencyMs)
		}
	}
	for i := range c.Histogram {
		if c.Histogram[i].Upper == nil || p.LatencyMs < *c.Histogram[i].Upper {
			c.Histogram[i].Count++
			break
		}
	}
	var difference *float64
	if compare {
		if c.previous != nil {
			difference = number(math.Abs(p.LatencyMs - *c.previous))
			c.differenceTotal += *difference
			c.pairs++
			c.CurrentJitter = difference
		}
		c.previous = number(p.LatencyMs)
	}
	return difference
}

func (c *counter) finish() Summary {
	c.Availability = rate(c.Successful, c.Attempts)
	if c.pairs > 0 {
		c.Jitter = number(c.differenceTotal / float64(c.pairs))
	}
	return c.Summary
}

type trendCounter struct {
	Trend
	pairs           int
	differenceTotal float64
}

type timeline struct {
	from, to time.Time
	buckets  []trendCounter
}

func newTimeline(from, to time.Time) *timeline {
	t := &timeline{from: from, to: to, buckets: make([]trendCounter, TrendBuckets)}
	span := to.Sub(from)
	for i := range t.buckets {
		t.buckets[i].Timestamp = from.Add(time.Duration(float64(span) * float64(i) / TrendBuckets))
	}
	return t
}

func (t *timeline) index(timestamp time.Time) int {
	i := 0
	if span := t.to.Sub(t.from); span > 0 {
		i = int(float64(timestamp.Sub(t.from)) / float64(span) * TrendBuckets)
	}
	if i < 0 {
		i = 0
	}
	if i >= TrendBuckets {
		i = TrendBuckets - 1
	}
	return i
}

func (t *timeline) mark(e anomaly.Event) {
	b := &t.buckets[t.index(e.CycleTimestamp)]
	b.AnomalyCount++
	if b.AnomalyLatency == nil || e.ObservedLatency > *b.AnomalyLatency {
		b.AnomalyLatency = number(e.ObservedLatency)
	}
}

func (t *timeline) observe(m store.Measurement, difference *float64) {
	i := t.index(m.SnapshotTimestamp)
	b := &t.buckets[i]
	b.Attempts++
	if m.Probe.Success {
		b.Successful++
		if b.Average == nil {
			b.Average = number(m.Probe.LatencyMs)
		} else {
			*b.Average += (m.Probe.LatencyMs - *b.Average) / float64(b.Successful)
		}
	}
	if difference != nil {
		b.differenceTotal += *difference
		b.pairs++
	}
}

func (t *timeline) points() []Trend {
	points := make([]Trend, len(t.buckets))
	for i, b := range t.buckets {
		b.Availability = rate(b.Successful, b.Attempts)
		if b.pairs > 0 {
			b.Jitter = number(b.differenceTotal / float64(b.pairs))
		}
		points[i] = b.Trend
	}
	return points
}
