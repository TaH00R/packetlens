package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/store"
)

type Handler struct {
	store *store.MemoryStore
}

func NewHandler(s *store.MemoryStore) *Handler {
	return &Handler{
		store: s,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"service":   "packetlens",
		"timestamp": time.Now(),
	})
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := h.store.GetAll()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(results),
		"results": results,
	})
}

type ProbeStats struct {
	Total      int     `json:"total"`
	Successful int     `json:"successful"`
	Failed     int     `json:"failed"`
	AvgLatency float64 `json:"avg_latency_ms"`
	MinLatency float64 `json:"min_latency_ms"`
	MaxLatency float64 `json:"max_latency_ms"`
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := h.store.GetAll()

	stats := make(map[string]*ProbeStats)

	for _, result := range results {
		s, exists := stats[result.Type]

		if !exists {
			s = &ProbeStats{}
			stats[result.Type] = s
		}

		s.Total++

		if result.Success {
			s.Successful++

			s.AvgLatency += result.LatencyMs

			if s.Successful == 1 || result.LatencyMs < s.MinLatency {
				s.MinLatency = result.LatencyMs
			}

			if result.LatencyMs > s.MaxLatency {
				s.MaxLatency = result.LatencyMs
			}
		} else {
			s.Failed++
		}
	}

	for _, s := range stats {
		if s.Successful > 0 {
			s.AvgLatency /= float64(s.Successful)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_results": len(results),
		"probes":        stats,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return
	}
}
