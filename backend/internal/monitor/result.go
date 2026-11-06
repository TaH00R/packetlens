package monitor

import "time"

type ProbeResult struct {
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	LatencyMs float64   `json:"latency_ms"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
