package api

import (
	"net/http"

	"github.com/TaH00R/packetlens/backend/internal/store"
)

func NewServer(s *store.MemoryStore) *http.Server {
	handler := NewHandler(s)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", handler.Health)
	mux.HandleFunc("/api/metrics", handler.Metrics)
	mux.HandleFunc("/api/stats", handler.Stats)

	return &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * 1000000000,
	}
}
