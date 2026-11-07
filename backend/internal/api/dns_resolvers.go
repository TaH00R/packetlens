package api

import (
	"context"
	"net/http"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/analytics"
)

func (h *Handler) DNSResolvers(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	selected, to, err := analyticsQuery(r, true)
	if err != nil {
		queryError(w, err)
		return
	}
	if !h.reserveAnalytics(w) {
		return
	}
	defer func() { <-h.analyticsSlots }()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	report, err := analytics.BuildResolvers(ctx, h.store, selected, to, h.dnsResolvers)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, 200, report)
}
