package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestRoutesAndCORS(t *testing.T) {
	s := store.NewMemoryStore(10)
	server := NewServer(s, realtime.NewHub())
	for _, path := range []string{"/api/health", "/api/metrics", "/api/stats", "/api/snapshots"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			r := httptest.NewRequest(method, path, nil)
			r.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, r)
			want := http.StatusOK
			if method == http.MethodPost {
				want = http.StatusMethodNotAllowed
			}
			if w.Code != want || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
				t.Fatalf("%s %s: code=%d headers=%v", method, path, w.Code, w.Header())
			}
			if method == http.MethodGet && !json.Valid(w.Body.Bytes()) {
				t.Fatalf("invalid JSON: %s", w.Body.String())
			}
		}
	}
	for _, origin := range []string{"http://localhost:3000", "http://127.0.0.1:3000", "https://untrusted.example"} {
		r := httptest.NewRequest(http.MethodOptions, "/api/snapshots", nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if origin == "https://untrusted.example" {
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("untrusted origin allowed")
			}
		} else if w.Code != http.StatusNoContent {
			t.Fatalf("preflight status: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	var metrics struct {
		Results []monitor.ProbeResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &metrics); err != nil || metrics.Results == nil {
		t.Fatalf("empty metrics must be an array: %s", w.Body.String())
	}
}

func TestStats(t *testing.T) {
	for _, tc := range []struct {
		name    string
		results []monitor.ProbeResult
		want    ProbeStats
	}{
		{name: "empty"},
		{name: "success", results: []monitor.ProbeResult{{Type: "tcp", Success: true, LatencyMs: 10}, {Type: "tcp", Success: true, LatencyMs: 30}}, want: ProbeStats{Total: 2, Successful: 2, AvgLatency: 20, MinLatency: 10, MaxLatency: 30}},
		{name: "failure", results: []monitor.ProbeResult{{Type: "tcp", LatencyMs: 3000}}, want: ProbeStats{Total: 1, Failed: 1}},
		{name: "mixed", results: []monitor.ProbeResult{{Type: "tcp", Success: true, LatencyMs: 10}, {Type: "tcp", LatencyMs: 3000}}, want: ProbeStats{Total: 2, Successful: 1, Failed: 1, AvgLatency: 10, MinLatency: 10, MaxLatency: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewMemoryStore(10)
			s.AddSnapshot(monitor.Snapshot{Timestamp: time.Now(), TCP: tc.results})
			w := httptest.NewRecorder()
			NewHandler(s).Stats(w, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
			var response struct {
				Probes map[string]ProbeStats `json:"probes"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Probes["tcp"] != tc.want {
				t.Fatalf("got %+v, want %+v", response.Probes["tcp"], tc.want)
			}
		})
	}
}
