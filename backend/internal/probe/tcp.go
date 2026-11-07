package probe

import (
	"context"
	"net"
	"time"
)

type TCPResult struct {
	Target    string  `json:"target"`
	LatencyMs float64 `json:"latency_ms"`
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
}

func CheckTCP(ctx context.Context, target string) TCPResult {
	return CheckTCPWithTimeout(ctx, target, 3*time.Second)
}

func CheckTCPWithTimeout(ctx context.Context, target string, timeout time.Duration) TCPResult {
	result := TCPResult{Target: target}

	dialer := net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()

	conn, err := dialer.DialContext(ctx, "tcp", target)

	result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000

	if err != nil {
		result.Error = err.Error()
		return result
	}

	defer conn.Close()

	result.Success = true
	return result
}
