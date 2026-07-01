package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/adp/panel/internal/db"
)

type healthHandler struct {
	db      *sql.DB
	version string
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// healthz is liveness: the process is up.
func (h *healthHandler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: h.version})
}

// readyz is readiness: the database is reachable and migrated.
func (h *healthHandler) readyz(w http.ResponseWriter, r *http.Request) {
	if err := db.Ready(r.Context(), h.db); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "down", Version: h.version})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: h.version})
}
