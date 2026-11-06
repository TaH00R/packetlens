package api

import (
	"net/http"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func NewServer(
	s store.Store,
	hub *realtime.Hub,
) *http.Server {
	handler := NewHandler(s)

	wsHandler := NewWebSocketHandler(hub)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", handler.Health)
	mux.HandleFunc("/api/metrics", handler.Metrics)
	mux.HandleFunc("/api/stats", handler.Stats)
	mux.Handle("/api/ws", wsHandler)
	mux.HandleFunc("/api/snapshots", handler.Snapshots)

	return &http.Server{
		Addr:              ":8080",
		Handler:           localCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// Match the frontend origins accepted by the WebSocket endpoint.
func localCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		switch r.Header.Get("Origin") {
		case "http://localhost:3000", "http://127.0.0.1:3000":
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
