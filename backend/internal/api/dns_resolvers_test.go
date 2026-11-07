package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestResolverAPI(t *testing.T) {
	s := store.NewMemoryStore(10)
	at := time.Now().UTC()
	_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at, DNSComparisons: []monitor.DNSComparison{{ProbeResult: monitor.ProbeResult{Type: "dns", Target: "example.test", Timestamp: at, Success: false, Error: "DNS query timed out", LatencyMs: 30}, Resolver: "127.0.0.1:53", Outcome: "timeout"}}})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(s, realtime.NewHub(), &TargetAdmin{DNSResolvers: []string{"127.0.0.1:53"}})
	request := func(method, path string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != status || !json.Valid(w.Body.Bytes()) {
			t.Fatal(w.Code, w.Body.String())
		}
		return w
	}
	w := request("GET", "/api/dns/resolvers?range=all", 200)
	var report struct {
		Configured []string `json:"configured_resolvers"`
		Groups     []struct {
			Failed   int      `json:"failed"`
			Timeouts int      `json:"timeouts"`
			Average  *float64 `json:"avg_latency_ms"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || len(report.Configured) != 1 || len(report.Groups) != 1 || report.Groups[0].Average != nil || report.Groups[0].Timeouts != 1 {
		t.Fatal("resolver response", err)
	}
	for _, query := range []string{"?range=bad", "?range=all&range=5m", "?to=bad", "?resolver=1.1.1.1:53", "?target=example.test"} {
		request("GET", "/api/dns/resolvers"+query, 400)
	}
	request("POST", "/api/dns/resolvers", 405)
	s.Close()
	request("GET", "/api/dns/resolvers", 503)
}
