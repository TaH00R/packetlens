package monitor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/probe"
	"github.com/TaH00R/packetlens/backend/internal/target"
)

func configs(n int) []target.Config {
	out := []target.Config{}
	for i := 0; i < n; i++ {
		out = append(out, target.Config{ID: fmt.Sprintf("%032x", i+1), Type: "tcp", Address: fmt.Sprintf("host%d:443", i), Enabled: true, IntervalMs: 5000, TimeoutMs: 3000})
	}
	return out
}

func TestDNSResolverFanoutUsesSharedPool(t *testing.T) {
	c := configs(10)
	for i := range c {
		c[i].Type = "dns"
	}
	var active, peak atomic.Int32
	measure := func() {
		n := active.Add(1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
	}
	m := NewConfiguredMonitor(c, func(context.Context, target.Config) ProbeResult { measure(); return ProbeResult{Success: true} }, "127.0.0.1:53", "127.0.0.2:53")
	m.resolverProbe = func(context.Context, string, string, time.Duration) probe.ResolverResult {
		measure()
		return probe.ResolverResult{DNSResult: probe.DNSResult{Success: true, LatencyMs: 3}, Outcome: "success"}
	}
	if !m.runCycle(context.Background(), c) {
		t.Fatal("missing cycle")
	}
	s := receive(t, m)
	if len(s.DNS) != 10 || len(s.DNSComparisons) != 20 || peak.Load() > MaxConcurrentProbes {
		t.Fatal("incorrect bounded fanout", len(s.DNS), len(s.DNSComparisons), peak.Load())
	}
	for _, p := range s.DNSComparisons {
		if p.Type != "dns" || p.Outcome != "success" || p.Resolver == "" {
			t.Fatal(p)
		}
	}
}
func receive(t *testing.T, m *Monitor) Snapshot {
	t.Helper()
	select {
	case s := <-m.Results:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("missing snapshot")
		return Snapshot{}
	}
}
func TestBoundedCompleteCycle(t *testing.T) {
	started := make(chan struct{}, 32)
	release := make(chan struct{})
	var concurrent, peak atomic.Int32
	m := NewConfiguredMonitor(nil, func(ctx context.Context, c target.Config) ProbeResult {
		n := concurrent.Add(1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		concurrent.Add(-1)
		return ProbeResult{Success: true}
	})
	done := make(chan bool, 1)
	go func() { done <- m.runCycle(context.Background(), configs(24)) }()
	for i := 0; i < 8; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker missing")
		}
	}
	select {
	case <-m.Results:
		t.Fatal("partial snapshot published")
	default:
	}
	if peak.Load() > MaxConcurrentProbes {
		t.Fatal("unbounded probes")
	}
	close(release)
	s := receive(t, m)
	if !<-done || len(s.TCP) != 24 || len(s.DNS) != 0 {
		t.Fatal("incomplete cycle", s)
	}
}
func TestReconfigurationDuringCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	old := configs(1)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	m := NewConfiguredMonitor(old, func(ctx context.Context, c target.Config) ProbeResult {
		if c.Address == old[0].Address {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		return ProbeResult{Success: true}
	})
	go m.Start(ctx)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("initial probe missing")
	}
	next := configs(1)
	next[0].Address = "new:443"
	m.Reconfigure(next)
	next[0].Address = "external mutation"
	close(release)
	if s := receive(t, m); s.TCP[0].Target != old[0].Address {
		t.Fatal("in-flight config mutated")
	}
	if s := receive(t, m); s.TCP[0].Target != "new:443" {
		t.Fatal("reconfiguration not applied")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.Reconfigure(nil)
			}
		}()
	}
	wg.Wait()
	cancel()
	select {
	case _, ok := <-m.Results:
		if ok {
			for range m.Results {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("monitor did not shut down")
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	c := configs(1)
	c[0].TimeoutMs = 20
	m := NewConfiguredMonitor(nil, func(ctx context.Context, c target.Config) ProbeResult {
		<-ctx.Done()
		return ProbeResult{Error: ctx.Err().Error()}
	})
	if !m.runCycle(context.Background(), c) {
		t.Fatal("timeout cycle lost")
	}
	s := receive(t, m)
	if s.TCP[0].Success || s.TCP[0].Error == "" {
		t.Fatal("timeout became a success")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m.runCycle(ctx, c) {
		t.Fatal("canceled cycle published")
	}
	select {
	case <-m.Results:
		t.Fatal("cancelled sample published")
	default:
	}
}
func TestPerTargetIntervalsAndDisabledTargets(t *testing.T) {
	c := configs(3)
	c[0].IntervalMs = 20
	c[1].IntervalMs = 5000
	c[2].Enabled = false
	m := NewConfiguredMonitor(c, func(context.Context, target.Config) ProbeResult { return ProbeResult{Success: true} })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Start(ctx)
	counts := map[string]int{}
	for i := 0; i < 4; i++ {
		for _, p := range receive(t, m).TCP {
			counts[p.Target]++
		}
	}
	if counts[c[0].Address] < 3 || counts[c[1].Address] != 1 || counts[c[2].Address] != 0 {
		t.Fatal("wrong per-target cadence", counts)
	}
}
