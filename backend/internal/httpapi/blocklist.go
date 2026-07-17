package httpapi

import (
	"net/http"
	"strconv"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/store"
	syncpkg "github.com/adp/panel/internal/sync"
)

// blocklistHandler manages the global forbidden-domain list. Saving it re-pushes
// config to every node so xray/sing-box drop connections to the listed domains.
type blocklistHandler struct {
	sync  *syncpkg.Service
	store *store.Store
}

type blocklistDTO struct {
	Domains string `json:"domains"` // raw, newline-delimited (one domain per line)
	Count   int    `json:"count"`   // number of valid, deduplicated domains
}

func (h *blocklistHandler) get(w http.ResponseWriter, r *http.Request) {
	raw, err := h.sync.Blocklist(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load blocklist")
		return
	}
	writeJSON(w, http.StatusOK, blocklistDTO{Domains: raw, Count: len(syncpkg.ParseBlocklist(raw))})
}

func (h *blocklistHandler) put(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domains string `json:"domains"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.sync.SetBlocklist(r.Context(), in.Domains); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save blocklist")
		return
	}
	count := len(syncpkg.ParseBlocklist(in.Domains))
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "blocklist.update", "blocklist", 0,
		`{"count":`+strconv.Itoa(count)+`}`)
	writeJSON(w, http.StatusOK, blocklistDTO{Domains: in.Domains, Count: count})
}
