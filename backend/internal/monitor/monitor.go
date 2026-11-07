package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/probe"
	"github.com/TaH00R/packetlens/backend/internal/target"
)

const MaxConcurrentProbes = 8

type ProbeFunc func(context.Context, target.Config) ProbeResult
type Monitor struct {
	Results       chan Snapshot
	mu            sync.Mutex
	configs       []target.Config
	changed       chan struct{}
	probe         ProbeFunc
	resolvers     []string
	resolverProbe func(context.Context, string, string, time.Duration) probe.ResolverResult
}

// NewMonitor preserves the original constructor for callers with static targets.
func NewMonitor(interval time.Duration, targets, domains []string) *Monitor {
	configs := []target.Config{}
	for _, g := range []struct {
		kind      string
		addresses []string
	}{{"tcp", targets}, {"dns", domains}} {
		for _, address := range g.addresses {
			configs = append(configs, target.Config{ID: fmt.Sprintf("%032x", len(configs)+1), Type: g.kind, Address: address, Enabled: true, IntervalMs: int(interval / time.Millisecond), TimeoutMs: 3000})
		}
	}
	return NewConfiguredMonitor(configs, nil)
}

// run may be injected by deterministic tests. Production uses context-aware probes.
func NewConfiguredMonitor(configs []target.Config, run ProbeFunc, resolvers ...string) *Monitor {
	if run == nil {
		run = runProbe
	}
	m := &Monitor{Results: make(chan Snapshot, 100), changed: make(chan struct{}, 1), probe: run}
	m.resolvers = append([]string{}, resolvers...)
	m.resolverProbe = probe.CheckDNSResolver
	m.Reconfigure(configs)
	return m
}
func (m *Monitor) Reconfigure(configs []target.Config) {
	m.mu.Lock()
	m.configs = append([]target.Config{}, configs...)
	m.mu.Unlock()
	select {
	case m.changed <- struct{}{}:
	default:
	}
}
func (m *Monitor) configuration() []target.Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]target.Config{}, m.configs...)
}

type scheduled struct {
	config target.Config
	next   time.Time
}

// The scheduling loop owns due times. Reconfiguration swaps an immutable set;
// it never mutates an in-flight cycle. Missed intervals are not replayed.
func (m *Monitor) Start(ctx context.Context) {
	defer close(m.Results)
	schedule := map[string]scheduled{}
	for {
		if ctx.Err() != nil {
			return
		}
		configs := m.configuration()
		now := time.Now()
		current := map[string]scheduled{}
		for _, c := range configs {
			if !c.Enabled {
				continue
			}
			entry, exists := schedule[c.ID]
			if !exists || entry.config != c {
				entry = scheduled{config: c, next: now}
			}
			current[c.ID] = entry
		}
		schedule = current
		due := []target.Config{}
		wait := time.Hour
		for _, c := range configs {
			entry, ok := schedule[c.ID]
			if !ok {
				continue
			}
			if !entry.next.After(now) {
				due = append(due, c)
				entry.next = now.Add(time.Duration(c.IntervalMs) * time.Millisecond)
				schedule[c.ID] = entry
			}
			remaining := entry.next.Sub(now)
			if remaining < wait {
				wait = remaining
			}
		}
		if len(due) > 0 {
			if !m.runCycle(ctx, due) {
				return
			}
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-m.changed:
			timer.Stop()
		case <-timer.C:
		}
	}
}
func (m *Monitor) runCycle(ctx context.Context, configs []target.Config) bool {
	type job struct {
		config   target.Config
		resolver string
	}
	tasks := []job{}
	for _, c := range configs {
		tasks = append(tasks, job{config: c})
		if c.Type == "dns" {
			for _, endpoint := range m.resolvers {
				tasks = append(tasks, job{config: c, resolver: endpoint})
			}
		}
	}
	results := make([]ProbeResult, len(tasks))
	comparisons := make([]DNSComparison, len(tasks))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := min(MaxConcurrentProbes, len(tasks))
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					continue
				}
				c := tasks[i].config
				probeCtx, cancel := context.WithTimeout(ctx, time.Duration(c.TimeoutMs)*time.Millisecond)
				var p ProbeResult
				if tasks[i].resolver == "" {
					p = m.probe(probeCtx, c)
				} else {
					r := m.resolverProbe(probeCtx, c.Address, tasks[i].resolver, time.Duration(c.TimeoutMs)*time.Millisecond)
					p = ProbeResult{LatencyMs: r.LatencyMs, Success: r.Success, Error: r.Error}
					comparisons[i] = DNSComparison{Resolver: tasks[i].resolver, Outcome: r.Outcome}
				}
				cancel()
				p.Type = c.Type
				p.Target = c.Address
				p.Timestamp = time.Now()
				results[i] = p
			}
		}()
	}
	for i := range tasks {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return false
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return false
	}
	snapshot := Snapshot{Timestamp: time.Now(), TCP: []ProbeResult{}, DNS: []ProbeResult{}}
	for i, p := range results {
		if tasks[i].resolver != "" {
			comparisons[i].ProbeResult = p
			snapshot.DNSComparisons = append(snapshot.DNSComparisons, comparisons[i])
			continue
		}
		if p.Type == "tcp" {
			snapshot.TCP = append(snapshot.TCP, p)
		} else {
			snapshot.DNS = append(snapshot.DNS, p)
		}
	}
	select {
	case m.Results <- snapshot:
		return true
	case <-ctx.Done():
		return false
	}
}
func runProbe(ctx context.Context, c target.Config) ProbeResult {
	timeout := time.Duration(c.TimeoutMs) * time.Millisecond
	if c.Type == "tcp" {
		p := probe.CheckTCPWithTimeout(ctx, c.Address, timeout)
		return ProbeResult{LatencyMs: p.LatencyMs, Success: p.Success, Error: p.Error}
	}
	p := probe.CheckDNSWithTimeout(ctx, c.Address, timeout)
	return ProbeResult{LatencyMs: p.LatencyMs, Success: p.Success, Error: p.Error}
}
