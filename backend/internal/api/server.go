package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/realtime"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func NewServer(
	s store.Store,
	hub *realtime.Hub,
	admin ...*TargetAdmin,
) *http.Server {
	handler := NewHandler(s)

	wsHandler := NewWebSocketHandler(hub)

	mux := http.NewServeMux()
	var settings *TargetAdmin
	if len(admin) > 0 {
		settings = admin[0]
	}
	if settings != nil {
		handler.dnsResolvers = append([]string{}, settings.DNSResolvers...)
	}
	targets := newTargetHandler(settings)
	mux.Handle("/api/targets", targets)
	mux.Handle("/api/targets/", targets)

	mux.HandleFunc("/api/export/measurements.csv", handler.ExportMeasurements)
	mux.HandleFunc("/api/export/dns-resolvers.csv", handler.ExportResolvers)
	mux.HandleFunc("/api/export/incidents.csv", handler.ExportIncidents)
	mux.HandleFunc("/api/health", handler.Health)
	mux.HandleFunc("/api/metrics", handler.Metrics)
	mux.HandleFunc("/api/stats", handler.Stats)
	mux.HandleFunc("/api/analytics", handler.Analytics)
	mux.HandleFunc("/api/dns/resolvers", handler.DNSResolvers)
	mux.HandleFunc("/api/anomalies", handler.Anomalies)
	mux.HandleFunc("/api/incidents", handler.Incidents)
	mux.HandleFunc("/api/correlations", handler.Correlations)
	mux.HandleFunc("/api/availability", handler.Availability)
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
			if r.URL.Path == "/api/targets" || strings.HasPrefix(r.URL.Path, "/api/targets/") {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
