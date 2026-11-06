package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/incident"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestIncidentHTTP(t *testing.T) {
	s := store.NewMemoryStore(100)
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		stamp := at.Add(time.Duration(i) * time.Second)
		if _, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: stamp, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: stamp}}}); err != nil {
			t.Fatal(err)
		}
	}
	server := NewServer(s, realtime.NewHub())
	request := func(query string, method string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/incidents"+query, nil)
		r.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != status || !json.Valid(w.Body.Bytes()) || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatalf("incident HTTP %s: %d %s", query, w.Code, w.Body.String())
		}
		return w
	}
	end := url.QueryEscape(at.Add(time.Minute).Format(time.RFC3339))
	w := request("?range=5m&to="+end+"&status=open&type=tcp&target=a", http.MethodGet, http.StatusOK)
	var response struct {
		Count     int              `json:"count"`
		Open      int              `json:"open_count"`
		Recovered int              `json:"recovered_count"`
		Events    []incident.Event `json:"incidents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Count != 1 || response.Open != 1 || response.Recovered != 0 || response.Events[0].DurationMs != nil {
		t.Fatalf("incident contract: %s %v", w.Body.String(), err)
	}
	for _, query := range []string{"?range=bad", "?status=unknown", "?status=", "?status=open&status=recovered", "?type=icmp", "?limit=201", "?before=bad", "?from=2026-11-07T00:00:00Z", "?to=invalid", "?unknown=value"} {
		request(query, http.MethodGet, http.StatusBadRequest)
	}
	request("", http.MethodPost, http.StatusMethodNotAllowed)
	s.Close()
	request("", http.MethodGet, http.StatusServiceUnavailable)
}
