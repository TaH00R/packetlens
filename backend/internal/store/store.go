package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/resolver"
)

const DefaultLimit = 1000
const MaxLimit = 1000
const CleanupBatchSize = 500
const timestampLayout = "2006-01-02T15:04:05.000000000Z"

var ErrInvalidQuery = errors.New("invalid history query")

// Filter uses snapshot timestamps, including both boundaries. Probe filters
// select matching cycles; QuerySnapshots always returns each complete cycle.
type Filter struct {
	From   *time.Time
	To     *time.Time
	Type   string
	Target string
}

type Query struct {
	Filter
	Limit  int
	Before string
}

type Page struct {
	Snapshots  []monitor.Snapshot
	NextCursor string
}

type ProbeStats struct {
	Total      int     `json:"total"`
	Successful int     `json:"successful"`
	Failed     int     `json:"failed"`
	AvgLatency float64 `json:"avg_latency_ms"`
	MinLatency float64 `json:"min_latency_ms"`
	MaxLatency float64 `json:"max_latency_ms"`
}

type Statistics struct {
	TotalResults int                    `json:"total_results"`
	Probes       map[string]*ProbeStats `json:"probes"`
}

// Store keeps database details out of collection and HTTP handling. All reads
// are bounded pages or database aggregates; methods respect caller cancellation.
type Store interface {
	SaveSnapshot(context.Context, monitor.Snapshot) (monitor.Snapshot, error)
	QuerySnapshots(context.Context, Query) (Page, error)
	Stats(context.Context, Filter) (Statistics, error)
	HistoryBounds(context.Context) (*time.Time, *time.Time, error)
	WalkMeasurements(context.Context, Filter, bool, func(Measurement) error) error
	WalkDNSComparisons(context.Context, Filter, func(time.Time, monitor.DNSComparison) error) error
	QueryAnomalies(context.Context, AnomalyQuery) (AnomalyPage, error)
	QueryIncidents(context.Context, IncidentQuery) (IncidentPage, error)
	WalkIncidents(context.Context, Filter, func(incident.Event) error) error
	QueryCorrelations(context.Context, EpisodeQuery) (CorrelationPage, error)
	WalkAnomalyStarts(context.Context, Filter, func(anomaly.Event) error) error
	DeleteBefore(context.Context, time.Time) (int64, error)
	Ping(context.Context) error
	Close() error
}

func (f Filter) Validate() error {
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return fmt.Errorf("%w: from must not be after to", ErrInvalidQuery)
	}
	if f.Type != "" && f.Type != "tcp" && f.Type != "dns" {
		return fmt.Errorf("%w: type must be tcp or dns", ErrInvalidQuery)
	}
	if len(f.Target) > 512 {
		return fmt.Errorf("%w: target is too long", ErrInvalidQuery)
	}
	for _, t := range []*time.Time{f.From, f.To} {
		if t != nil && (t.UTC().Year() < 1 || t.UTC().Year() > 9999) {
			return fmt.Errorf("%w: timestamp year must be between 1 and 9999", ErrInvalidQuery)
		}
	}
	return nil
}

func (q Query) normalized() (Query, error) {
	if err := q.Filter.Validate(); err != nil {
		return q, err
	}
	if q.Limit == 0 {
		q.Limit = DefaultLimit
	}
	if q.Limit < 1 || q.Limit > MaxLimit {
		return q, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidQuery, MaxLimit)
	}
	if q.Before != "" {
		if _, err := decodeCursor(q.Before); err != nil {
			return q, err
		}
	}
	return q, nil
}

type cursor struct {
	Timestamp string `json:"t"`
	ID        string `json:"id"`
}

func encodeCursor(s monitor.Snapshot) string {
	data, _ := json.Marshal(cursor{Timestamp: formatTime(s.Timestamp), ID: s.ID})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (cursor, error) {
	var c cursor
	if len(value) > 256 {
		return c, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(data, &c) != nil {
		return c, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
	}
	t, err := time.Parse(timestampLayout, c.Timestamp)
	if err != nil || t.Year() < 1 || formatTime(t) != c.Timestamp || !validID(c.ID) {
		return c, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
	}
	return c, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(timestampLayout) }

func validID(id string) bool {
	decoded, err := hex.DecodeString(id)
	return len(id) == 32 && err == nil && hex.EncodeToString(decoded) == id
}

func prepareSnapshot(s monitor.Snapshot) (monitor.Snapshot, error) {
	if s.ID == "" {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return s, fmt.Errorf("generate snapshot ID: %w", err)
		}
		s.ID = hex.EncodeToString(id[:])
	}
	if !validID(s.ID) || s.Timestamp.IsZero() || s.Timestamp.UTC().Year() < 1 || s.Timestamp.UTC().Year() > 9999 {
		return s, errors.New("invalid snapshot ID or timestamp")
	}
	for _, group := range []struct {
		typ    string
		probes []monitor.ProbeResult
	}{{"tcp", s.TCP}, {"dns", s.DNS}} {
		for _, p := range group.probes {
			if p.Type != group.typ || p.Target == "" || len(p.Target) > 512 || math.IsNaN(p.LatencyMs) || math.IsInf(p.LatencyMs, 0) || p.LatencyMs < 0 || p.Timestamp.IsZero() || p.Timestamp.UTC().Year() < 1 || p.Timestamp.UTC().Year() > 9999 {
				return s, errors.New("invalid probe result")
			}
		}
	}
	if len(s.DNSComparisons) > 96 {
		return s, errors.New("too many DNS comparisons")
	}
	for _, p := range s.DNSComparisons {
		endpoint, err := resolver.Normalize(p.Resolver)
		if err != nil || endpoint != p.Resolver || p.Type != "dns" || !resolver.ValidOutcome(p.Outcome, p.Success) || p.Target == "" || len(p.Target) > 512 || p.Timestamp.IsZero() || p.Timestamp.UTC().Year() < 1 || p.Timestamp.UTC().Year() > 9999 || math.IsNaN(p.LatencyMs) || math.IsInf(p.LatencyMs, 0) || p.LatencyMs < 0 {
			return s, errors.New("invalid DNS comparison")
		}
	}
	return cloneSnapshot(s), nil
}

func matchesProbe(f Filter, p monitor.ProbeResult) bool {
	return (f.Type == "" || p.Type == f.Type) && (f.Target == "" || p.Target == f.Target)
}

func matchesTime(f Filter, t time.Time) bool {
	return (f.From == nil || !t.Before(*f.From)) && (f.To == nil || !t.After(*f.To))
}

func pageSnapshots(snapshots []monitor.Snapshot, q Query) Page {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Timestamp.Equal(snapshots[j].Timestamp) {
			return snapshots[i].ID > snapshots[j].ID
		}
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})
	page := Page{Snapshots: snapshots}
	if len(snapshots) > q.Limit {
		page.Snapshots = snapshots[:q.Limit]
		page.NextCursor = encodeCursor(page.Snapshots[len(page.Snapshots)-1])
	}
	// Preserve the existing chronological response order within every page.
	for i, j := 0, len(page.Snapshots)-1; i < j; i, j = i+1, j-1 {
		page.Snapshots[i], page.Snapshots[j] = page.Snapshots[j], page.Snapshots[i]
	}
	return page
}
