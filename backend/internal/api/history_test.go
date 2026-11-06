package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestSQLiteHistoryHTTP(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		snapshot := monitor.Snapshot{Timestamp: at.Add(time.Duration(i) * time.Second), TCP: []monitor.ProbeResult{
			{Type: "tcp", Target: "127.0.0.1:443", Success: i != 2, LatencyMs: float64(10 + i), Timestamp: at},
		}}
		if _, err := s.SaveSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = store.OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	server := NewServer(s, realtime.NewHub())
	get := func(path string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != status || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	w := get("/api/snapshots?limit=2", http.StatusOK)
	var first struct {
		Count      int                `json:"count"`
		Snapshots  []monitor.Snapshot `json:"snapshots"`
		NextCursor string             `json:"next_cursor"`
		HasMore    bool               `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.Count != 2 || len(first.Snapshots) != 2 || first.Snapshots[0].ID == "" || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("invalid first page: %+v", first)
	}
	w = get("/api/snapshots?limit=2&before="+url.QueryEscape(first.NextCursor), http.StatusOK)
	var next struct {
		Count   int  `json:"count"`
		HasMore bool `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &next); err != nil || next.Count != 1 || next.HasMore {
		t.Fatalf("second page: %s %v", w.Body.String(), err)
	}
	params := url.Values{"from": {at.Format(time.RFC3339Nano)}, "to": {at.Add(time.Second).Format(time.RFC3339Nano)}, "type": {"tcp"}, "target": {"127.0.0.1:443"}}
	w = get("/api/stats?"+params.Encode(), http.StatusOK)
	var stats store.Statistics
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || stats.TotalResults != 2 || stats.Probes["tcp"].AvgLatency != 10.5 {
		t.Fatalf("range stats: %s %v", w.Body.String(), err)
	}
	w = get("/api/metrics?type=dns", http.StatusOK)
	if !strings.Contains(w.Body.String(), `"results":[]`) {
		t.Fatalf("empty metrics: %s", w.Body.String())
	}
	for _, query := range []string{"limit=0", "limit=1001", "limit=-1", "limit=abc", "before=invalid", "type=icmp", "from=invalid", "from=2026-11-07T00:00:00Z&to=2026-11-06T00:00:00Z", "limit=1&limit=2", "unknown=value", "target=", "from=%ZZ"} {
		get("/api/snapshots?"+query, http.StatusBadRequest)
	}
	get("/api/stats?limit=10", http.StatusBadRequest)
	s.Close()
	for _, path := range []string{"/api/snapshots", "/api/metrics", "/api/stats", "/api/health"} {
		w := get(path, http.StatusServiceUnavailable)
		if strings.Contains(w.Body.String(), "sql:") || strings.Contains(w.Body.String(), "history.db") {
			t.Fatalf("database details leaked: %s", w.Body.String())
		}
	}
}

func TestDefaultHistoryLimit(t *testing.T) {
	s := store.NewMemoryStore(store.DefaultLimit + 2)
	ctx := context.Background()
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < store.DefaultLimit+1; i++ {
		if _, err := s.SaveSnapshot(ctx, monitor.Snapshot{ID: fmt.Sprintf("%032x", i+1), Timestamp: at.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	NewHandler(s).Snapshots(w, httptest.NewRequest(http.MethodGet, "/api/snapshots", nil))
	var response struct {
		Count   int  `json:"count"`
		HasMore bool `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Count != store.DefaultLimit || !response.HasMore {
		t.Fatalf("unbounded default history: count=%d err=%v", response.Count, err)
	}
}
