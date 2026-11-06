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
	Results  chan Snapshot
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
		Results:  make(chan Snapshot, 100),
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
	total := len(m.Targets) + len(m.Domains)

	results := make(chan ProbeResult, total)

	var wg sync.WaitGroup

	for _, target := range m.Targets {
		wg.Add(1)

		go func(target string) {
			defer wg.Done()

			result := probe.CheckTCP(ctx, target)

			results <- ProbeResult{
				Type:      "tcp",
				Target:    result.Target,
				LatencyMs: result.LatencyMs,
				Success:   result.Success,
				Error:     result.Error,
				Timestamp: time.Now(),
			}
		}(target)
	}

	for _, domain := range m.Domains {
		wg.Add(1)

		go func(domain string) {
			defer wg.Done()

			result := probe.CheckDNS(ctx, domain)

			results <- ProbeResult{
				Type:      "dns",
				Target:    result.Domain,
				LatencyMs: result.LatencyMs,
				Success:   result.Success,
				Error:     result.Error,
				Timestamp: time.Now(),
			}
		}(domain)
	}

	wg.Wait()
	close(results)

	snapshot := Snapshot{
		Timestamp: time.Now(),
		TCP:       make([]ProbeResult, 0),
		DNS:       make([]ProbeResult, 0),
	}

	for result := range results {
		switch result.Type {
		case "tcp":
			snapshot.TCP = append(snapshot.TCP, result)
		case "dns":
			snapshot.DNS = append(snapshot.DNS, result)
		}
	}

	select {
	case m.Results <- snapshot:
	case <-ctx.Done():
	}
}
