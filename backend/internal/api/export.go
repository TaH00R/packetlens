package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

type exportLimit struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (l *exportLimit) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.last.IsZero() {
		l.tokens = 6
		l.last = now
	}
	l.tokens = min(6, l.tokens+now.Sub(l.last).Seconds()/10)
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

func parseExportQuery(r *http.Request) (store.Filter, string, time.Time, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return store.Filter{}, "", time.Time{}, err
	}
	for key := range values {
		switch key {
		case "range", "to", "type", "target":
		default:
			return store.Filter{}, "", time.Time{}, fmt.Errorf("%w: unknown export parameter %s", store.ErrInvalidQuery, key)
		}
	}
	q, selected, to, err := parseEpisodeQuery(r)
	return q.Filter, selected, to, err
}
func (h *Handler) ExportMeasurements(w http.ResponseWriter, r *http.Request) {
	h.exportCSV(w, r, "measurements")
}
func (h *Handler) ExportResolvers(w http.ResponseWriter, r *http.Request) {
	h.exportCSV(w, r, "dns-resolvers")
}
func (h *Handler) ExportIncidents(w http.ResponseWriter, r *http.Request) {
	h.exportCSV(w, r, "incidents")
}

func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request, kind string) {
	if !requireGET(w, r) {
		return
	}
	f, selected, to, err := parseExportQuery(r)
	if err != nil {
		queryError(w, err)
		return
	}
	if kind == "dns-resolvers" && f.Type == "tcp" {
		queryError(w, fmt.Errorf("%w: resolver exports support DNS only", store.ErrInvalidQuery))
		return
	}
	if !h.exportLimit.allow() {
		w.Header().Set("Retry-After", "10")
		writeJSON(w, 429, map[string]string{"error": "export rate limit reached; retry shortly"})
		return
	}
	select {
	case h.exportSlots <- struct{}{}:
		defer func() { <-h.exportSlots }()
	default:
		w.Header().Set("Retry-After", "10")
		writeJSON(w, 429, map[string]string{"error": "another export is running; retry shortly"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	headers := []string{"timestamp", "snapshot_timestamp", "target", "probe_type", "latency_ms", "success", "error"}
	if kind == "dns-resolvers" {
		headers = append(headers, "resolver", "outcome")
	}
	if kind == "incidents" {
		headers = []string{"id", "probe_type", "target", "status", "started_at", "cycle_timestamp", "detected_at", "last_failure_at", "last_observed_at", "recovered_at", "elapsed_span_ms", "failed_probes", "recovery_successes", "has_monitoring_gaps", "first_error", "last_error", "failure_threshold", "recovery_threshold", "max_gap_ms"}
	}
	stream := newCSVStream(w, fmt.Sprintf("packetlens-%s-%s-%s.csv", kind, selected, to.Format("20060102T150405Z")), headers)
	defer stream.clearDeadline()
	switch kind {
	case "measurements":
		err = h.store.WalkMeasurements(ctx, f, false, func(m store.Measurement) error { return stream.write(measurementRow(m.SnapshotTimestamp, m.Probe)) })
	case "dns-resolvers":
		err = h.store.WalkDNSComparisons(ctx, f, func(at time.Time, p monitor.DNSComparison) error {
			return stream.write(append(measurementRow(at, p.ProbeResult), p.Resolver, p.Outcome))
		})
	case "incidents":
		err = h.store.WalkIncidents(ctx, f, func(e incident.Event) error { return stream.write(incidentRow(e)) })
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = stream.finish()
	}
	if err != nil {
		log.Printf("CSV export failed | dataset=%s | rows=%d | error=%v", kind, stream.rows, err)
		if !stream.started {
			storageError(w, r, err)
			return
		}
		// Headers may already be on the wire. Go's designated abort sentinel
		// terminates the HTTP stream so clients reject the incomplete download;
		// appending JSON or a success-shaped CSV footer would corrupt the file.
		panic(http.ErrAbortHandler)
	}
}
func measurementRow(cycle time.Time, p monitor.ProbeResult) []string {
	return []string{csvTime(p.Timestamp), csvTime(cycle), p.Target, p.Type, csvNumber(p.LatencyMs), strconv.FormatBool(p.Success), p.Error}
}
func incidentRow(e incident.Event) []string {
	status, recovered, duration := "open", "", ""
	if e.RecoveredAt != nil {
		status = "recovered"
		recovered = csvTime(*e.RecoveredAt)
		duration = csvNumber(*e.DurationMs)
	}
	return []string{e.ID, e.Type, e.Target, status, csvTime(e.StartedAt), csvTime(e.CycleTimestamp), csvTime(e.DetectedAt), csvTime(e.LastFailureAt), csvTime(e.LastObservedAt), recovered, duration, strconv.Itoa(e.FailedProbes), strconv.Itoa(e.RecoverySuccesses), strconv.FormatBool(e.HasMonitoringGaps), e.FirstError, e.LastError, strconv.Itoa(e.Policy.FailureThreshold), strconv.Itoa(e.Policy.RecoveryThreshold), strconv.FormatInt(e.Policy.MaxGap.Milliseconds(), 10)}
}
func csvTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func csvNumber(n float64) string { return strconv.FormatFloat(n, 'g', -1, 64) }

type csvStream struct {
	w          http.ResponseWriter
	csv        *csv.Writer
	controller *http.ResponseController
	filename   string
	header     []string
	started    bool
	rows       int
}

func newCSVStream(w http.ResponseWriter, filename string, header []string) *csvStream {
	return &csvStream{w: w, csv: csv.NewWriter(w), controller: http.NewResponseController(w), filename: filename, header: header}
}
func (s *csvStream) begin() error {
	if s.started {
		return nil
	}
	if err := s.deadline(); err != nil {
		return err
	}
	s.w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	s.w.Header().Set("Content-Disposition", `attachment; filename="`+s.filename+`"`)
	s.w.Header().Set("Cache-Control", "no-store")
	s.w.Header().Set("X-Content-Type-Options", "nosniff")
	s.started = true
	return s.csv.Write(s.header)
}
func (s *csvStream) write(row []string) error {
	if err := s.begin(); err != nil {
		return err
	}
	if err := s.csv.Write(row); err != nil {
		return err
	}
	s.rows++
	if s.rows%128 == 0 {
		return s.flush()
	}
	return nil
}
func (s *csvStream) deadline() error {
	err := s.controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}
func (s *csvStream) clearDeadline() { _ = s.controller.SetWriteDeadline(time.Time{}) }
func (s *csvStream) flush() error {
	if err := s.deadline(); err != nil {
		return err
	}
	s.csv.Flush()
	if err := s.csv.Error(); err != nil {
		return err
	}
	err := s.controller.Flush()
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}
func (s *csvStream) finish() error {
	if err := s.begin(); err != nil {
		return err
	}
	return s.flush()
}
