package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/analytics"
	"github.com/TaH00R/packetlens/backend/internal/store"
)

func analyticsQuery(r *http.Request, allowRange bool) (string, time.Time, error) {
	selected, to := "15m", time.Now().UTC()
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return selected, to, err
	}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return selected, to, fmt.Errorf("%w: expected one nonempty value for %s", store.ErrInvalidQuery, key)
		}
		switch key {
		case "range":
			if !allowRange {
				return selected, to, fmt.Errorf("%w: availability uses fixed windows", store.ErrInvalidQuery)
			}
			selected = entries[0]
			if _, valid := analytics.Ranges[selected]; !valid && selected != "all" {
				return selected, to, fmt.Errorf("%w: unsupported range", store.ErrInvalidQuery)
			}
		case "to":
			t, err := time.Parse(time.RFC3339Nano, entries[0])
			if err != nil || t.UTC().Year() < 1970 || t.UTC().Year() > 9998 {
				return selected, to, fmt.Errorf("%w: to must be an RFC3339 timestamp between years 1970 and 9998", store.ErrInvalidQuery)
			}
			to = t.UTC()
		default:
			return selected, to, fmt.Errorf("%w: unknown parameter %s", store.ErrInvalidQuery, key)
		}
	}
	return selected, to, nil
}

func (h *Handler) Analytics(w http.ResponseWriter, r *http.Request) {
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
	report, err := analytics.Build(ctx, h.store, selected, to)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	_, to, err := analyticsQuery(r, false)
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
	report, err := analytics.Availability(ctx, h.store, to)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// Leave database connections available for collection and short HTTP reads.
func (h *Handler) reserveAnalytics(w http.ResponseWriter) bool {
	select {
	case h.analyticsSlots <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "15")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "analytics is busy; retry shortly"})
		return false
	}
}
