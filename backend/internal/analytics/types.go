package analytics

import "time"

const TrendBuckets = 60
const MaxTargets = 128

type Bucket struct {
	Lower float64  `json:"lower_ms"`
	Upper *float64 `json:"upper_ms"`
	Count int      `json:"count"`
}

type Summary struct {
	Type          string     `json:"type"`
	Target        string     `json:"target,omitempty"`
	Attempts      int        `json:"attempts"`
	Successful    int        `json:"successful"`
	Failed        int        `json:"failed"`
	Availability  *float64   `json:"availability"`
	Average       *float64   `json:"avg_latency_ms"`
	Minimum       *float64   `json:"min_latency_ms"`
	Maximum       *float64   `json:"max_latency_ms"`
	P50           *float64   `json:"p50_ms"`
	P90           *float64   `json:"p90_ms"`
	P95           *float64   `json:"p95_ms"`
	P99           *float64   `json:"p99_ms"`
	Jitter        *float64   `json:"jitter_ms"`
	CurrentJitter *float64   `json:"current_jitter_ms"`
	FirstObserved *time.Time `json:"first_observed"`
	LastObserved  *time.Time `json:"last_observed"`
	Histogram     []Bucket   `json:"histogram"`
}

type Trend struct {
	AnomalyCount   int       `json:"anomaly_count"`
	AnomalyLatency *float64  `json:"anomaly_latency_ms"`
	Timestamp      time.Time `json:"timestamp"`
	Attempts       int       `json:"attempts"`
	Successful     int       `json:"successful"`
	Average        *float64  `json:"avg_latency_ms"`
	Jitter         *float64  `json:"jitter_ms"`
	Availability   *float64  `json:"availability"`
}

type Series struct {
	Type   string  `json:"type"`
	Target string  `json:"target,omitempty"`
	Points []Trend `json:"points"`
}

type Period struct {
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	Groups       []Summary `json:"groups"`
	Targets      []Summary `json:"targets"`
	Trends       []Series  `json:"trends"`
	TargetTrends []Series  `json:"target_trends"`
}

type Report struct {
	Range        string     `json:"range"`
	AsOf         time.Time  `json:"as_of"`
	HistoryStart *time.Time `json:"history_start"`
	Current      Period     `json:"current"`
	Previous     *Period    `json:"previous"`
}

type AvailabilityWindow struct {
	Window        string     `json:"window"`
	From          time.Time  `json:"from"`
	To            time.Time  `json:"to"`
	Attempts      int        `json:"attempts"`
	Successful    int        `json:"successful"`
	Availability  *float64   `json:"availability"`
	FirstObserved *time.Time `json:"first_observed"`
	LastObserved  *time.Time `json:"last_observed"`
	Trend         []Trend    `json:"trend"`
}

type AvailabilityTarget struct {
	Type                 string               `json:"type"`
	Target               string               `json:"target,omitempty"`
	ConsecutiveSuccesses int                  `json:"consecutive_successes"`
	ConsecutiveFailures  int                  `json:"consecutive_failures"`
	LastSuccess          *time.Time           `json:"last_success"`
	LastFailure          *time.Time           `json:"last_failure"`
	Windows              []AvailabilityWindow `json:"windows"`
}

type AvailabilityReport struct {
	AsOf         time.Time            `json:"as_of"`
	HistoryStart *time.Time           `json:"history_start"`
	Groups       []AvailabilityTarget `json:"groups"`
	Targets      []AvailabilityTarget `json:"targets"`
}
