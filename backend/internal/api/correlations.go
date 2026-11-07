package api

import (
	"context"
	"net/http"
	"time"
)

func (h *Handler) Correlations(w http.ResponseWriter, r *http.Request) {
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
	page, err := h.store.QueryCorrelations(ctx, q)
	if err != nil {
		storageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": selected, "as_of": to, "count": len(page.Events), "correlations": page.Events, "has_more": page.NextCursor != "", "next_cursor": page.NextCursor})
}
