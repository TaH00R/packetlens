package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/monitor"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

type Handler struct{ store store.Store }

func NewHandler(s store.Store) *Handler { return &Handler{store: s} }

func requireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status, code := "healthy", http.StatusOK
	if err := h.store.Ping(ctx); err != nil {
		log.Printf("[HTTP] Database health check failed: %v", err)
		status, code = "degraded", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": status, "service": "packetlens", "timestamp": time.Now()})
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	q, err := parseQuery(r, true)
	if err != nil {
		queryError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.store.QuerySnapshots(ctx, q)
	if err != nil {
		storageError(w, r, err)
		return
	}
	results := make([]monitor.ProbeResult, 0)
	for _, snapshot := range page.Snapshots {
		for _, probes := range [][]monitor.ProbeResult{snapshot.TCP, snapshot.DNS} {
			for _, p := range probes {
				if (q.Type == "" || q.Type == p.Type) && (q.Target == "" || q.Target == p.Target) {
					results = append(results, p)
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(results), "results": results,
		"next_cursor": page.NextCursor, "has_more": page.NextCursor != "",
	})
}

// Snapshots preserves complete cycles and chronological order within each page.
func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	q, err := parseQuery(r, true)
	if err != nil {
		queryError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.store.QuerySnapshots(ctx, q)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(page.Snapshots), "snapshots": page.Snapshots,
		"next_cursor": page.NextCursor, "has_more": page.NextCursor != "",
	})
}

type ProbeStats = store.ProbeStats

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	q, err := parseQuery(r, false)
	if err != nil {
		queryError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	stats, err := h.store.Stats(ctx, q.Filter)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func parseQuery(r *http.Request, pagination bool) (store.Query, error) {
	q := store.Query{Limit: store.DefaultLimit}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return q, err
	}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return q, fmt.Errorf("%w: %s must have one nonempty value", store.ErrInvalidQuery, key)
		}
		value := entries[0]
		switch key {
		case "from", "to":
			t, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return q, fmt.Errorf("%w: %s must be an RFC3339 timestamp", store.ErrInvalidQuery, key)
			}
			if key == "from" {
				q.From = &t
			} else {
				q.To = &t
			}
		case "type":
			q.Type = value
		case "target":
			q.Target = value
		case "limit":
			if !pagination {
				return q, fmt.Errorf("%w: stats does not accept pagination", store.ErrInvalidQuery)
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > store.MaxLimit {
				return q, fmt.Errorf("%w: limit must be between 1 and %d", store.ErrInvalidQuery, store.MaxLimit)
			}
			q.Limit = n
		case "before":
			if !pagination {
				return q, fmt.Errorf("%w: stats does not accept pagination", store.ErrInvalidQuery)
			}
			q.Before = value
		default:
			return q, fmt.Errorf("%w: unknown parameter %s", store.ErrInvalidQuery, key)
		}
	}
	return q, q.Filter.Validate()
}

func queryError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func storageError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrInvalidQuery) {
		queryError(w, err)
		return
	}
	log.Printf("[HTTP] Storage operation failed | path=%s | error=%v", r.URL.Path, err)
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "history storage is temporarily unavailable"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[HTTP] Failed to encode JSON response: %v", err)
	}
}
