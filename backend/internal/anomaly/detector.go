// Package anomaly implements deterministic high-latency detection. It neither
// attributes root causes nor treats failed probes as latency measurements.
package anomaly

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
)

type Config struct {
	Window          int           `json:"window"`
	MinSamples      int           `json:"min_samples"`
	Multiplier      float64       `json:"mad_multiplier"`
	MinDeltaMs      float64       `json:"min_delta_ms"`
	MaxAge          time.Duration `json:"max_age_ns"`
	RecoverySamples int           `json:"recovery_samples"`
}

func DefaultConfig() Config {
	return Config{Window: 60, MinSamples: 20, Multiplier: 6, MinDeltaMs: 10, MaxAge: 15 * time.Minute, RecoverySamples: 3}
}

func (c Config) Validate() error {
	if c.Window < 3 || c.Window > 500 || c.MinSamples < 3 || c.MinSamples > c.Window {
		return fmt.Errorf("anomaly window must be 3..500 and minimum samples 3..window")
	}
	if math.IsNaN(c.Multiplier) || math.IsInf(c.Multiplier, 0) || c.Multiplier < 1 || c.Multiplier > 100 {
		return fmt.Errorf("anomaly MAD multiplier must be finite and 1..100")
	}
	if math.IsNaN(c.MinDeltaMs) || math.IsInf(c.MinDeltaMs, 0) || c.MinDeltaMs < .1 || c.MinDeltaMs > 60000 {
		return fmt.Errorf("anomaly minimum delta must be finite and 0.1..60000 ms")
	}
	if c.MaxAge < time.Minute || c.MaxAge > 24*time.Hour {
		return fmt.Errorf("anomaly maximum baseline age must be 1m..24h")
	}
	if c.RecoverySamples < 1 || c.RecoverySamples > 100 {
		return fmt.Errorf("anomaly recovery samples must be 1..100")
	}
	return nil
}

type Sample struct {
	Timestamp time.Time `json:"timestamp"`
	Latency   float64   `json:"latency_ms"`
}

type Event struct {
	ID               string     `json:"id"`
	Type             string     `json:"type"`
	Target           string     `json:"target"`
	Timestamp        time.Time  `json:"timestamp"`
	CycleTimestamp   time.Time  `json:"cycle_timestamp"`
	LastAnomalyAt    time.Time  `json:"last_anomaly_at"`
	LastObservedAt   time.Time  `json:"last_observed_at"`
	RecoveredAt      *time.Time `json:"recovered_at"`
	RecoveryCycle    *time.Time `json:"recovery_cycle"`
	ObservedLatency  float64    `json:"observed_latency_ms"`
	PeakLatency      float64    `json:"peak_latency_ms"`
	Baseline         float64    `json:"baseline_ms"`
	MAD              float64    `json:"mad_ms"`
	Threshold        float64    `json:"threshold_ms"`
	Severity         string     `json:"severity"`
	AnomalousSamples int        `json:"anomalous_samples"`
	Method           string     `json:"method"`
	Policy           Config     `json:"policy"`
	BaselineSamples  int        `json:"baseline_samples"`
}

type State struct {
	Samples      []Sample  `json:"samples"`
	Recovery     []Sample  `json:"recovery"`
	Active       *Event    `json:"active"`
	LastObserved time.Time `json:"last_observed"`
}

func median(values []float64) float64 {
	sort.Float64s(values)
	i := len(values) / 2
	if len(values)%2 == 1 {
		return values[i]
	}
	return values[i-1]/2 + values[i]/2
}

// Process is pure: it clones input slices and active event before changing them.
// A failed transaction can retry without advancing an in-memory baseline.
func Process(previous State, cfg Config, snapshotID string, ordinal int, cycle time.Time, p monitor.ProbeResult) (State, *Event) {
	if !previous.LastObserved.IsZero() && p.Timestamp.Before(previous.LastObserved) {
		return previous, nil
	}
	s := previous
	s.Samples = append([]Sample{}, previous.Samples...)
	s.Recovery = append([]Sample{}, previous.Recovery...)
	if previous.Active != nil {
		event := *previous.Active
		s.Active = &event
	}
	cutoff := p.Timestamp.Add(-cfg.MaxAge)
	kept := s.Samples[:0]
	for _, sample := range s.Samples {
		if !sample.Timestamp.Before(cutoff) {
			kept = append(kept, sample)
		}
	}
	s.Samples = kept
	if len(s.Samples) > cfg.Window {
		s.Samples = s.Samples[len(s.Samples)-cfg.Window:]
	}
	if !s.LastObserved.IsZero() && p.Timestamp.Sub(s.LastObserved) > cfg.MaxAge {
		s.Recovery = nil
	}
	s.LastObserved = p.Timestamp
	if s.Active != nil {
		e := s.Active
		e.LastObservedAt = p.Timestamp
		if !p.Success {
			s.Recovery = nil
			return s, e
		}
		if p.LatencyMs > e.Threshold {
			s.Recovery = nil
			e.AnomalousSamples++
			e.LastAnomalyAt = p.Timestamp
			if p.LatencyMs > e.PeakLatency {
				e.PeakLatency = p.LatencyMs
			}
			if p.LatencyMs >= e.Baseline+2*(e.Threshold-e.Baseline) {
				e.Severity = "critical"
			}
			return s, e
		}
		s.Recovery = append(s.Recovery, Sample{p.Timestamp, p.LatencyMs})
		if len(s.Recovery) >= e.Policy.RecoverySamples {
			recovered, recoveryCycle := p.Timestamp, cycle
			e.RecoveredAt = &recovered
			e.RecoveryCycle = &recoveryCycle
			s.Samples = append(s.Samples, s.Recovery...)
			if len(s.Samples) > cfg.Window {
				s.Samples = s.Samples[len(s.Samples)-cfg.Window:]
			}
			s.Recovery = nil
			s.Active = nil
		}
		return s, e
	}
	if !p.Success {
		return s, nil
	}
	if len(s.Samples) >= cfg.MinSamples {
		values := make([]float64, len(s.Samples))
		for i, v := range s.Samples {
			values[i] = v.Latency
		}
		baseline := median(values)
		for i, v := range values {
			values[i] = math.Abs(v - baseline)
		}
		mad := median(values)
		threshold := baseline + math.Max(cfg.MinDeltaMs, cfg.Multiplier*mad)
		if p.LatencyMs > threshold {
			severity := "warning"
			if p.LatencyMs >= baseline+2*(threshold-baseline) {
				severity = "critical"
			}
			e := &Event{ID: snapshotID + "-" + strconv.Itoa(ordinal), Type: p.Type, Target: p.Target, Timestamp: p.Timestamp, CycleTimestamp: cycle,
				LastAnomalyAt: p.Timestamp, LastObservedAt: p.Timestamp, ObservedLatency: p.LatencyMs, PeakLatency: p.LatencyMs,
				Baseline: baseline, MAD: mad, Threshold: threshold, Severity: severity, AnomalousSamples: 1, Method: "rolling-median-mad-v1", Policy: cfg, BaselineSamples: len(s.Samples)}
			s.Active = e
			s.Recovery = nil
			return s, e
		}
	}
	s.Samples = append(s.Samples, Sample{p.Timestamp, p.LatencyMs})
	if len(s.Samples) > cfg.Window {
		s.Samples = s.Samples[len(s.Samples)-cfg.Window:]
	}
	return s, nil
}
