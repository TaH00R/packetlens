package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/analytics"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestAnalyticsHTTP(t *testing.T) {
	s := store.NewMemoryStore(100)
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		stamp := at.Add(-time.Duration(i) * time.Minute)
		_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: stamp, Success: i != 2, LatencyMs: float64(10 + i)}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	w := httptest.NewRecorder()
	h.Analytics(w, httptest.NewRequest(http.MethodGet, "/api/analytics?range=5m&to="+url.QueryEscape(at.Format(time.RFC3339)), nil))
	var report analytics.Report
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || w.Code != http.StatusOK || report.Current.Groups[0].Attempts != 3 || report.Current.Groups[0].P95 == nil || *report.Current.Groups[0].P95 != 11 {
		t.Fatalf("analytics contract: %d %s %v", w.Code, w.Body.String(), err)
	}
	w = httptest.NewRecorder()
	h.Availability(w, httptest.NewRequest(http.MethodGet, "/api/availability?to="+url.QueryEscape(at.Format(time.RFC3339)), nil))
	var available analytics.AvailabilityReport
	if err := json.Unmarshal(w.Body.Bytes(), &available); err != nil || w.Code != http.StatusOK || len(available.Targets) != 1 || available.Targets[0].ConsecutiveSuccesses != 2 {
		t.Fatalf("availability contract: %d %s %v", w.Code, w.Body.String(), err)
	}
	for _, query := range []string{"range=bad", "range=5m&range=1h", "to=invalid", "to=0001-01-01T00:00:00Z", "unknown=1", "range="} {
		w = httptest.NewRecorder()
		h.Analytics(w, httptest.NewRequest(http.MethodGet, "/api/analytics?"+query, nil))
		if w.Code != http.StatusBadRequest || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("invalid query accepted: %s %d", query, w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.Availability(w, httptest.NewRequest(http.MethodGet, "/api/availability?range=5m", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatal("availability ignored invalid range")
	}
	for _, handler := range []http.HandlerFunc{h.Analytics, h.Availability} {
		w = httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, "/", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatal("method accepted")
		}
	}
	h.analyticsSlots <- struct{}{}
	h.analyticsSlots <- struct{}{}
	w = httptest.NewRecorder()
	h.Analytics(w, httptest.NewRequest(http.MethodGet, "/api/analytics", nil))
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatal("unbounded analytics concurrency")
	}
	<-h.analyticsSlots
	<-h.analyticsSlots
	s.Close()
	for _, handler := range []http.HandlerFunc{h.Analytics, h.Availability} {
		w = httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatal("storage failure concealed")
		}
	}
}
