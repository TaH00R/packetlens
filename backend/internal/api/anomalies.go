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

// Episode feeds share range bounds, probe filters, and keyset pagination.
func parseEpisodeQuery(r *http.Request) (store.EpisodeQuery, string, time.Time, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return store.EpisodeQuery{}, "", time.Time{}, err
	}
	rangeValues, queryValues := url.Values{}, url.Values{}
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
		return store.EpisodeQuery{}, selected, to, err
	}
	if _, ok := queryValues["limit"]; !ok {
		queryValues.Set("limit", "50")
	}
	clone.URL.RawQuery = queryValues.Encode()
	q, err := parseQuery(clone, true)
	if err != nil {
		return store.EpisodeQuery{}, selected, to, err
	}
	if q.From != nil || q.To != nil {
		return store.EpisodeQuery{}, selected, to, fmt.Errorf("%w: episode time bounds use range and to", store.ErrInvalidQuery)
	}
	q.To = &to
	if selected != "all" {
		from := to.Add(-analytics.Ranges[selected])
		q.From = &from
	}
	return store.EpisodeQuery{Filter: q.Filter, Limit: q.Limit, Before: q.Before}, selected, to, nil
}

func (h *Handler) Anomalies(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	q, selected, to, err := parseEpisodeQuery(r)
	if err != nil {
		queryError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.store.QueryAnomalies(ctx, q)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": selected, "as_of": to, "count": len(page.Events), "anomalies": page.Events, "has_more": page.NextCursor != "", "next_cursor": page.NextCursor})
}
