package monitor

import "time"

type DNSComparison struct {
	ProbeResult
	Resolver string `json:"resolver"`
	Outcome  string `json:"outcome"`
}

type Snapshot struct {
	DNSComparisons []DNSComparison `json:"dns_comparisons,omitempty"`
	ID             string          `json:"id,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
	TCP            []ProbeResult   `json:"tcp"`
	DNS            []ProbeResult   `json:"dns"`
}
