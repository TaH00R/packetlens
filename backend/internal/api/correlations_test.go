package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/correlation"
	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func TestCorrelationHTTP(t *testing.T) {
	s := store.NewMemoryStore(10)
	at := time.Now().UTC()
	_, err := s.SaveSnapshot(context.Background(), monitor.Snapshot{Timestamp: at, TCP: []monitor.ProbeResult{{Type: "tcp", Target: "a", Timestamp: at}}, DNS: []monitor.ProbeResult{{Type: "dns", Target: "b", Timestamp: at}}})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(s, realtime.NewHub())
	request := func(query, method string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/correlations"+query, nil)
		r.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != status || !json.Valid(w.Body.Bytes()) || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatalf("%s %d %s", query, w.Code, w.Body.String())
		}
		return w
	}
	w := request("?range=all&type=dns&target=b", "GET", 200)
	var body struct {
		Count  int                 `json:"count"`
		Events []correlation.Event `json:"correlations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Count != 1 || len(body.Events[0].Members) != 2 {
		t.Fatal("invalid contract", err)
	}
	for _, q := range []string{"?range=bad", "?type=icmp", "?limit=201", "?before=bad", "?status=open", "?range=all&range=5m", "?to=bad", "?target="} {
		request(q, "GET", 400)
	}
	request("", http.MethodPost, 405)
	s.Close()
	request("", "GET", 503)
}
