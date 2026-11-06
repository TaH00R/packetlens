package monitor

import "time"

type Snapshot struct {
	Timestamp time.Time     `json:"timestamp"`
	TCP       []ProbeResult `json:"tcp"`
	DNS       []ProbeResult `json:"dns"`
}
