package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/probe"
)

type Monitor struct {
	Interval time.Duration
	Targets  []string
	Domains  []string
	Results  chan ProbeResult
}

func NewMonitor(
	interval time.Duration,
	targets []string,
	domains []string,
) *Monitor {
	return &Monitor{
		Interval: interval,
		Targets:  targets,
		Domains:  domains,
		Results:  make(chan ProbeResult, 100),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()
	defer close(m.Results)

	m.runProbes(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			m.runProbes(ctx)
		}
	}
}

func (m *Monitor) runProbes(ctx context.Context) {
	var wg sync.WaitGroup

	for _, target := range m.Targets {
		wg.Add(1)

		go func(target string) {
			defer wg.Done()

			result := probe.CheckTCP(ctx, target)

			probeResult := ProbeResult{
				Type:      "tcp",
				Target:    result.Target,
				LatencyMs: result.LatencyMs,
				Success:   result.Success,
				Error:     result.Error,
				Timestamp: time.Now(),
			}

			select {
			case m.Results <- probeResult:
			case <-ctx.Done():
			}
		}(target)
	}

	for _, domain := range m.Domains {
		wg.Add(1)

		go func(domain string) {
			defer wg.Done()

			result := probe.CheckDNS(ctx, domain)

			probeResult := ProbeResult{
				Type:      "dns",
				Target:    result.Domain,
				LatencyMs: result.LatencyMs,
				Success:   result.Success,
				Error:     result.Error,
				Timestamp: time.Now(),
			}

			select {
			case m.Results <- probeResult:
			case <-ctx.Done():
			}
		}(domain)
	}

	wg.Wait()
}
