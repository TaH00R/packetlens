package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/target"
)

type TargetAdmin struct {
	Manager      *target.Manager
	Token        string
	DNSResolvers []string
}
type targetHandler struct {
	admin  *TargetAdmin
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newTargetHandler(admin *TargetAdmin) http.Handler {
	return &targetHandler{admin: admin, tokens: 30, last: time.Now()}
}
func (h *targetHandler) allow() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	h.tokens = min(30, h.tokens+now.Sub(h.last).Seconds()/2)
	h.last = now
	if h.tokens < 1 {
		return false
	}
	h.tokens--
	return true
}
func targetError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func (h *targetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.allow() {
		w.Header().Set("Retry-After", "2")
		targetError(w, 429, "Target management rate limit reached; retry shortly")
		return
	}
	if h.admin == nil || h.admin.Manager == nil || h.admin.Token == "" {
		targetError(w, 503, "Target management is disabled. Configure PACKETLENS_ADMIN_TOKEN on the backend and restart it.")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://localhost:3000" && origin != "http://127.0.0.1:3000" {
		targetError(w, 403, "Origin is not allowed")
		return
	}
	expected := sha256.Sum256([]byte("Bearer " + h.admin.Token))
	actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		targetError(w, 401, "A valid admin token is required")
		return
	}
	if r.URL.RawQuery != "" {
		targetError(w, 400, "Query parameters are not supported")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/targets")
	collection := id == ""
	if !collection {
		if !strings.HasPrefix(id, "/") || !target.ValidID(id[1:]) {
			targetError(w, 400, "Invalid target ID")
			return
		}
		id = id[1:]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if collection && r.Method == http.MethodGet {
		configs := h.admin.Manager.List()
		writeJSON(w, 200, map[string]any{"targets": configs, "count": len(configs)})
		return
	}
	var err error
	switch {
	case collection && r.Method == http.MethodPost, !collection && r.Method == http.MethodPut:
		if r.Header.Get("Content-Type") != "application/json" {
			targetError(w, 415, "Content-Type must be application/json")
			return
		}
		// Pointer fields distinguish omitted required values from valid false/zero.
		var body struct {
			Type       *string `json:"type"`
			Address    *string `json:"address"`
			Enabled    *bool   `json:"enabled"`
			IntervalMs *int    `json:"interval_ms"`
			TimeoutMs  *int    `json:"timeout_ms"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || body.Type == nil || body.Address == nil || body.Enabled == nil || body.IntervalMs == nil || body.TimeoutMs == nil {
			targetError(w, 400, "Provide type, address, enabled, interval_ms and timeout_ms in one valid JSON object")
			return
		}
		c := target.Config{ID: id, Type: *body.Type, Address: *body.Address, Enabled: *body.Enabled, IntervalMs: *body.IntervalMs, TimeoutMs: *body.TimeoutMs}
		var saved target.Config
		saved, err = h.admin.Manager.Save(ctx, c, collection)
		if err == nil {
			status := 200
			if collection {
				status = 201
			}
			writeJSON(w, status, saved)
			return
		}
	case !collection && r.Method == http.MethodDelete:
		err = h.admin.Manager.Delete(ctx, id)
		if err == nil {
			writeJSON(w, 200, map[string]bool{"deleted": true})
			return
		}
	default:
		allow := "PUT, DELETE"
		if collection {
			allow = "GET, POST"
		}
		w.Header().Set("Allow", allow)
		targetError(w, 405, "Method not allowed")
		return
	}
	switch {
	case errors.Is(err, target.ErrInvalid):
		targetError(w, 400, err.Error())
	case errors.Is(err, target.ErrConflict):
		targetError(w, 409, err.Error())
	case errors.Is(err, target.ErrNotFound):
		targetError(w, 404, "Target not found")
	default:
		log.Printf("Target management failed | method=%s | error=%v", r.Method, err)
		targetError(w, 503, "Target configuration could not be saved; retry shortly")
	}
}
