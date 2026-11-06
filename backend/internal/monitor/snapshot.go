package monitor

import "time"

type Snapshot struct {
	ID        string        `json:"id,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	TCP       []ProbeResult `json:"tcp"`
	DNS       []ProbeResult `json:"dns"`
}
