package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
	"github.com/TaH00R/packetlens/backend/internal/target"
)

const testAdminToken = "test-only-secret-with-at-least-32-characters"

func TestTargetManagementHTTP(t *testing.T) {
	s, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "targets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var scheduled []target.Config
	m, err := target.NewManager(context.Background(), s, func(c []target.Config) { scheduled = c })
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(s, realtime.NewHub(), &TargetAdmin{Manager: m, Token: testAdminToken})
	request := func(method, path, body, token, origin string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if status != 204 && !json.Valid(w.Body.Bytes()) {
			t.Fatal("non JSON response")
		}
		return w
	}
	request("GET", "/api/targets", "", "", "", 401)
	request("POST", "/api/targets", "{}", "bad", "", 401)
	request("GET", "/api/targets", "", testAdminToken, "https://evil.example", 403)
	w := request("GET", "/api/targets", "", testAdminToken, "http://localhost:3000", 200)
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"count":4`) {
		t.Fatal("list contract", w.Body.String())
	}
	body := `{"type":"tcp","address":" Localhost:00443 ","enabled":true,"interval_ms":10000,"timeout_ms":5000}`
	w = request("POST", "/api/targets", body, testAdminToken, "http://localhost:3000", 201)
	var saved target.Config
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || saved.Address != "localhost:443" || len(scheduled) != 5 {
		t.Fatal("create contract", err)
	}
	path := "/api/targets/" + saved.ID
	request("POST", "/api/targets", body, testAdminToken, "", 409)
	updated := strings.ReplaceAll(body, `"enabled":true`, `"enabled":false`)
	request("PUT", path, updated, testAdminToken, "", 200)
	if m.List()[4].Enabled {
		t.Fatal("disable not applied")
	}
	request("POST", "/api/targets", `{"type":"tcp"}`, testAdminToken, "", 400)
	request("POST", "/api/targets", body+` {}`, testAdminToken, "", 400)
	request("POST", "/api/targets", strings.Replace(body, `"tcp"`, `"icmp"`, 1), testAdminToken, "", 400)
	request("POST", "/api/targets", strings.Replace(body, `"address"`, `"unknown"`, 1), testAdminToken, "", 400)
	request("GET", "/api/targets?limit=2", "", testAdminToken, "", 400)
	request("GET", path, "", testAdminToken, "", 405)
	request("PUT", "/api/targets/not-an-id", body, testAdminToken, "", 400)
	request("PUT", "/api/targets/ffffffffffffffffffffffffffffffff", body, testAdminToken, "", 404)
	request("DELETE", path, "", testAdminToken, "", 200)
	request("DELETE", path, "", testAdminToken, "", 404)
	r := httptest.NewRequest("OPTIONS", "/api/targets", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	w = httptest.NewRecorder()
	server.Handler.ServeHTTP(w, r)
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatal("management preflight rejected")
	}
	disabled := NewServer(s, realtime.NewHub())
	w = httptest.NewRecorder()
	disabled.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/targets", nil))
	if w.Code != 503 {
		t.Fatal("management enabled without token")
	}
	s.Close()
	request("POST", "/api/targets", body, testAdminToken, "", 503)
	if len(m.List()) != 4 {
		t.Fatal("failed store changed runtime settings")
	}
}
func TestTargetRateAndBodyLimits(t *testing.T) {
	h := newTargetHandler(nil)
	for i := 0; i < 30; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/targets", nil))
		if w.Code != 503 {
			t.Fatal(w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/targets", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("missing rate limit")
	}
	s, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "body.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, err := target.NewManager(context.Background(), s, func([]target.Config) {})
	if err != nil {
		t.Fatal(err)
	}
	h = newTargetHandler(&TargetAdmin{Manager: m, Token: testAdminToken})
	for _, contentType := range []string{"text/plain", "application/json"} {
		r := httptest.NewRequest("POST", "/api/targets", strings.NewReader(`{"address":"`+strings.Repeat("a", 5000)+`"}`))
		r.Header.Set("Content-Type", contentType)
		r.Header.Set("Authorization", "Bearer "+testAdminToken)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		expected := 400
		if contentType == "text/plain" {
			expected = 415
		}
		if w.Code != expected {
			t.Fatal("body validation", w.Code)
		}
	}
}
