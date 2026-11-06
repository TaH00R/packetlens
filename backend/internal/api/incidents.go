package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/TaH00R/packetlens/backend/internal/store"
)

func (h *Handler) Incidents(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		queryError(w, err)
		return
	}
	status := "all"
	if entries, ok := values["status"]; ok {
		if len(entries) != 1 || (entries[0] != "all" && entries[0] != "open" && entries[0] != "recovered") {
			queryError(w, fmt.Errorf("%w: status must be all, open, or recovered", store.ErrInvalidQuery))
			return
		}
		status = entries[0]
		values.Del("status")
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	clone.URL = &u
	clone.URL.RawQuery = values.Encode()
	q, selected, to, err := parseEpisodeQuery(clone)
	if err != nil {
		queryError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.store.QueryIncidents(ctx, store.IncidentQuery{EpisodeQuery: q, Status: status})
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": selected, "as_of": to, "status": status, "count": len(page.Events), "incidents": page.Events,
		"open_count": page.OpenCount, "recovered_count": page.RecoveredCount, "has_more": page.NextCursor != "", "next_cursor": page.NextCursor})
}
