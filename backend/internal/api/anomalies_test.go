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
	"github.com/TaH00R/packetlens/backend/internal/anomaly"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestAnomaliesHTTPAndTrendMarkers(t *testing.T) {
	s := store.NewMemoryStore(100)
	base := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 21; i++ {
		stamp := base.Add(time.Duration(i) * time.Second)
		latency := 10.0
		if i == 20 {
			latency = 100
		}
		_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: stamp, LatencyMs: latency, Success: true}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(s)
	end := url.QueryEscape(base.Add(time.Minute).Format(time.RFC3339))
	w := httptest.NewRecorder()
	h.Anomalies(w, httptest.NewRequest(http.MethodGet, "/api/anomalies?range=5m&to="+end, nil))
	var response struct {
		Count  int             `json:"count"`
		Events []anomaly.Event `json:"anomalies"`
		More   bool            `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != http.StatusOK || response.Count != 1 || len(response.Events) != 1 || response.Events[0].Baseline != 10 || response.More {
		t.Fatalf("anomaly contract: %d %s %v", w.Code, w.Body.String(), err)
	}
	w = httptest.NewRecorder()
	h.Analytics(w, httptest.NewRequest(http.MethodGet, "/api/analytics?range=5m&to="+end, nil))
	var report analytics.Report
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || w.Code != http.StatusOK {
		t.Fatalf("analytics response: %v %s", err, w.Body.String())
	}
	count := 0
	for _, p := range report.Current.Trends[0].Points {
		count += p.AnomalyCount
		if p.AnomalyCount > 0 && (p.AnomalyLatency == nil || *p.AnomalyLatency != 100) {
			t.Fatal("marker must show onset measurement")
		}
	}
	if count != 1 {
		t.Fatalf("onset markers=%d", count)
	}
	for _, query := range []string{"range=bad", "before=bad", "limit=201", "type=icmp", "from=2026-11-07T00:00:00Z", "target=", "range=5m&range=all", "to=invalid"} {
		w = httptest.NewRecorder()
		h.Anomalies(w, httptest.NewRequest(http.MethodGet, "/api/anomalies?"+query, nil))
		if w.Code != http.StatusBadRequest || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("invalid query accepted: %s %d", query, w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.Anomalies(w, httptest.NewRequest(http.MethodPost, "/api/anomalies", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("POST allowed")
	}
	s.Close()
	w = httptest.NewRecorder()
	h.Anomalies(w, httptest.NewRequest(http.MethodGet, "/api/anomalies", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal("storage failure hidden")
	}
}
