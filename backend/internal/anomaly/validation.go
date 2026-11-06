package anomaly

import (
	"fmt"
	"math"
)

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

func (e Event) Validate() error {
	if (e.Type != "tcp" && e.Type != "dns") || e.Target == "" || len(e.Target) > 512 || e.Timestamp.IsZero() || e.CycleTimestamp.IsZero() || e.LastAnomalyAt.Before(e.Timestamp) || e.LastObservedAt.Before(e.LastAnomalyAt) {
		return fmt.Errorf("invalid anomaly identity or timestamps")
	}
	if (e.RecoveredAt == nil) != (e.RecoveryCycle == nil) || (e.RecoveredAt != nil && e.RecoveredAt.Before(e.LastObservedAt)) {
		return fmt.Errorf("invalid anomaly recovery timestamps")
	}
	if !finite(e.Baseline) || !finite(e.MAD) || !finite(e.Threshold) || !finite(e.ObservedLatency) || !finite(e.PeakLatency) || e.Threshold <= e.Baseline || e.ObservedLatency <= e.Threshold || e.PeakLatency < e.ObservedLatency {
		return fmt.Errorf("invalid anomaly latency statistics")
	}
	if (e.Severity != "warning" && e.Severity != "critical") || e.Method != "rolling-median-mad-v1" || e.AnomalousSamples < 1 {
		return fmt.Errorf("invalid anomaly method or severity")
	}
	if err := e.Policy.Validate(); err != nil {
		return err
	}
	if e.BaselineSamples < e.Policy.MinSamples || e.BaselineSamples > e.Policy.Window {
		return fmt.Errorf("invalid anomaly baseline count")
	}
	return nil
}

func (s State) Validate() error {
	if s.LastObserved.IsZero() || len(s.Samples) > 500 || len(s.Recovery) > 100 {
		return fmt.Errorf("invalid detector state bounds")
	}
	for _, samples := range [][]Sample{s.Samples, s.Recovery} {
		for i, v := range samples {
			if !finite(v.Latency) || v.Timestamp.IsZero() || v.Timestamp.After(s.LastObserved) || (i > 0 && v.Timestamp.Before(samples[i-1].Timestamp)) {
				return fmt.Errorf("invalid detector samples")
			}
		}
	}
	if s.Active == nil {
		if len(s.Recovery) > 0 {
			return fmt.Errorf("recovery samples without an active episode")
		}
		return nil
	}
	if err := s.Active.Validate(); err != nil {
		return err
	}
	if s.Active.RecoveredAt != nil || !s.Active.LastObservedAt.Equal(s.LastObserved) || len(s.Recovery) >= s.Active.Policy.RecoverySamples {
		return fmt.Errorf("invalid active anomaly state")
	}
	return nil
}
