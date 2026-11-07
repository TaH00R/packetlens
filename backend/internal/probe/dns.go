package probe

import (
	"context"
	"net"
	"time"
)

type DNSResult struct {
	Domain    string   `json:"domain"`
	LatencyMs float64  `json:"latency_ms"`
	Success   bool     `json:"success"`
	Addresses []string `json:"addresses,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func CheckDNS(ctx context.Context, domain string) DNSResult {
	return CheckDNSWithTimeout(ctx, domain, 3*time.Second)
}

func CheckDNSWithTimeout(ctx context.Context, domain string, timeout time.Duration) DNSResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := DNSResult{
		Domain: domain,
	}

	resolver := net.DefaultResolver

	start := time.Now()

	addresses, err := resolver.LookupHost(ctx, domain)

	result.LatencyMs = float64(
		time.Since(start).Microseconds(),
	) / 1000

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Addresses = addresses

	return result
}
