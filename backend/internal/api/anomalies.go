package api

import (
"context"
"net/http"
"net/url"
"time"

"github.com/TaH00R/packetlens/backend/internal/analytics"
"github.com/TaH00R/packetlens/backend/internal/store"
)

func (h *Handler) Anomalies(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		queryError(w, err)
		return
	}
	rangeValues := url.Values{}
	queryValues := url.Values{}
	for key, entries := range values {
		if key == "range" || key == "to" {
			rangeValues[key] = entries
		} else {
			queryValues[key] = entries
		}
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	clone.URL = &u
	clone.URL.RawQuery = rangeValues.Encode()
	selected, to, err := analyticsQuery(clone, true)
	if err != nil {
		queryError(w, err)
		return
	}
	if _, ok := queryValues["limit"]; !ok {
		queryValues.Set("limit", "50")
	}
	clone.URL.RawQuery = queryValues.Encode()
	q, err := parseQuery(clone, true)
	if err != nil {
		queryError(w, err)
		return
	}
	// The range selector owns time bounds; a second independent from is invalid.
	if q.From != nil || q.To != nil {
		queryError(w, store.ErrInvalidQuery)
		return
	}
	q.To = &to
	if selected != "all" {
		from := to.Add(-analytics.Ranges[selected])
		q.From = &from
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.store.QueryAnomalies(ctx, store.AnomalyQuery{Filter: q.Filter, Limit: q.Limit, Before: q.Before})
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": selected, "as_of": to, "count": len(page.Events), "anomalies": page.Events, "has_more": page.NextCursor != "", "next_cursor": page.NextCursor})
}
