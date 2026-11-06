package api

import (
	"net/http"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func NewServer(
	s *store.MemoryStore,
	hub *realtime.Hub,
) *http.Server {
	handler := NewHandler(s)

	wsHandler := NewWebSocketHandler(hub)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", handler.Health)
	mux.HandleFunc("/api/metrics", handler.Metrics)
	mux.HandleFunc("/api/stats", handler.Stats)
	mux.Handle("/api/ws", wsHandler)

	return &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
