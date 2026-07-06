package httpapi

import (
	"net/http"

	"github.com/adp/panel/internal/sysinfo"
)

// systemHandler reports host metrics for the machine the panel runs on.
type systemHandler struct {
	version string
	domain  string
}

// systemDTO is the host snapshot plus panel identity fields.
type systemDTO struct {
	sysinfo.Info
	PanelVersion string `json:"panel_version"`
	Domain       string `json:"domain,omitempty"`
}

func (h *systemHandler) get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, systemDTO{
		Info:         sysinfo.Collect(),
		PanelVersion: h.version,
		Domain:       h.domain,
	})
}
