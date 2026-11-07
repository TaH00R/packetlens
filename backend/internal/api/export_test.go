package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestMeasurementCSVFiltersEscapingAndEmpty(t *testing.T) {
	s := store.NewMemoryStore(10)
	at := time.Date(2026, 11, 7, 12, 0, 0, 123, time.UTC)
	target := "quoted,\"target\n.test:443"
	detail := "failure, \"quoted\"\nsecond line"
	for i := 0; i < 3; i++ {
		stamp := at.Add(time.Duration(i) * time.Minute)
		_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, TCP: []monitor.ProbeResult{{Type: "tcp", Target: target, Timestamp: stamp, LatencyMs: 1.25, Success: i != 1, Error: detail}}, DNS: []monitor.ProbeResult{{Type: "dns", Target: "example.test", Timestamp: stamp, Success: true}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	params := url.Values{"range": {"5m"}, "to": {at.Add(time.Minute).Format(time.RFC3339Nano)}, "type": {"tcp"}, "target": {target}}
	w := httptest.NewRecorder()
	h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv?"+params.Encode(), nil))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv") || !strings.Contains(w.Header().Get("Content-Disposition"), "packetlens-measurements-5m-") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if err != nil || len(rows) != 3 || rows[1][2] != target || rows[1][6] != detail || rows[2][5] != "false" || rows[2][4] != "1.25" || rows[1][0] != at.Format(time.RFC3339Nano) {
		t.Fatal("CSV did not round trip", rows, err)
	}
	w = httptest.NewRecorder()
	h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv?range=all&target=missing", nil))
	rows, err = csv.NewReader(w.Body).ReadAll()
	if err != nil || len(rows) != 1 {
		t.Fatal("empty CSV not header only", rows, err)
	}
	for _, query := range []string{"?range=bad", "?range=all&range=5m", "?type=icmp", "?target=", "?before=bad", "?limit=1", "?from=2026-01-01T00:00:00Z", "?to=bad", "?unknown=x"} {
		w = httptest.NewRecorder()
		h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv"+query, nil))
		if w.Code != 400 {
			t.Fatal("invalid export accepted", query, w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.ExportMeasurements(w, httptest.NewRequest("POST", "/api/export/measurements.csv", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
func TestResolverAndIncidentCSV(t *testing.T) {
	s := store.NewMemoryStore(100)
	at := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 5; i++ {
		stamp := at.Add(time.Duration(i) * time.Second)
		success := i >= 3
		outcome := "timeout"
		if success {
			outcome = "success"
		}
		_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, DNS: []monitor.ProbeResult{{Type: "dns", Target: "example.test", Timestamp: stamp, Success: success, Error: "lookup failed"}}, DNSComparisons: []monitor.DNSComparison{{ProbeResult: monitor.ProbeResult{Type: "dns", Target: "example.test", Timestamp: stamp, Success: success, LatencyMs: 20, Error: "comparison failed"}, Resolver: "127.0.0.1:53", Outcome: outcome}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	for _, dataset := range []string{"incidents", "dns-resolvers"} {
		w := httptest.NewRecorder()
		path := "/api/export/" + dataset + ".csv?range=all&type=dns&target=example.test"
		if dataset == "incidents" {
			h.ExportIncidents(w, httptest.NewRequest("GET", path, nil))
		} else {
			h.ExportResolvers(w, httptest.NewRequest("GET", path, nil))
		}
		rows, err := csv.NewReader(w.Body).ReadAll()
		if err != nil || w.Code != 200 {
			t.Fatal(rows, err)
		}
		if dataset == "incidents" {
			if len(rows) != 2 || rows[1][3] != "recovered" || rows[1][10] != "4000" || rows[1][11] != "3" {
				t.Fatal("incident export", rows)
			}
		} else {
			if len(rows) != 6 || rows[1][7] != "127.0.0.1:53" || rows[1][8] != "timeout" {
				t.Fatal("resolver export", rows)
			}
		}
	}
	w := httptest.NewRecorder()
	h.ExportResolvers(w, httptest.NewRequest("GET", "/api/export/dns-resolvers.csv?type=tcp", nil))
	if w.Code != 400 {
		t.Fatal("TCP resolver export accepted")
	}
}

type exportFaultStore struct {
	store.Store
	count   int
	fail    error
	started chan struct{}
	release chan struct{}
}

func (s *exportFaultStore) WalkMeasurements(ctx context.Context, f store.Filter, ranked bool, visit func(store.Measurement) error) error {
	if s.started != nil {
		close(s.started)
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for i := 0; i < s.count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(store.Measurement{SnapshotTimestamp: time.Now(), Probe: monitor.ProbeResult{Type: "tcp", Target: "test:443", Timestamp: time.Now(), Success: true}}); err != nil {
			return err
		}
	}
	return s.fail
}
func TestExportFailuresAndConcurrency(t *testing.T) {
	for _, count := range []int{0, 200} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			h := NewHandler(&exportFaultStore{count: count, fail: errors.New("injected storage failure")})
			w := httptest.NewRecorder()
			defer func() {
				recovered := recover()
				if count == 0 {
					if recovered != nil || w.Code != 503 || !strings.Contains(w.Body.String(), "temporarily unavailable") {
						t.Fatal("initial failure not JSON", recovered, w.Code, w.Body.String())
					}
				} else if recovered != http.ErrAbortHandler {
					t.Fatal("partial CSV was not aborted", recovered)
				}
				if len(h.exportSlots) != 0 {
					t.Fatal("export slot leaked")
				}
			}()
			h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv", nil))
		})
	}
	fault := &exportFaultStore{started: make(chan struct{}), release: make(chan struct{})}
	h := NewHandler(fault)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ExportMeasurements(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/export/measurements.csv", nil))
	}()
	<-fault.started
	w := httptest.NewRecorder()
	h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("concurrent export not bounded")
	}
	close(fault.release)
	<-done
	limited := NewHandler(&exportFaultStore{})
	for i := 0; i < 7; i++ {
		w = httptest.NewRecorder()
		limited.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv", nil))
		expected := 200
		if i == 6 {
			expected = 429
		}
		if w.Code != expected {
			t.Fatal("rate limit", i, w.Code)
		}
	}
}
func TestLargeSQLiteExportAndCancellation(t *testing.T) {
	s, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "export.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 1005; i++ {
		stamp := at.Add(time.Duration(i) * time.Second)
		if _, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "local.test:443", Timestamp: stamp, Success: true}}}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	w := httptest.NewRecorder()
	h.ExportMeasurements(w, httptest.NewRequest("GET", "/api/export/measurements.csv?range=all", nil))
	reader := csv.NewReader(w.Body)
	count := 0
	for {
		_, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 1006 {
		t.Fatal("history page cap truncated export", count)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "/api/export/measurements.csv?range=all", nil).WithContext(ctx)
	w = httptest.NewRecorder()
	h.ExportMeasurements(w, r)
	if w.Code != 503 {
		t.Fatal("cancelled query continued")
	}
}

func TestPartialExportFailsHTTPTransfer(t *testing.T) {
	h := NewHandler(&exportFaultStore{count: 200, fail: errors.New("injected late read error")})
	server := httptest.NewServer(http.HandlerFunc(h.ExportMeasurements))
	defer server.Close()
	response, err := http.Get(server.URL + "?range=all")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if _, err := io.ReadAll(response.Body); err == nil {
		t.Fatal("partial HTTP export looked complete")
	}
}
