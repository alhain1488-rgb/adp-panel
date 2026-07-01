package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/store"
)

type serversHandler struct {
	svc   *servers.Service
	store *store.Store
}

// --- DTOs (mirror docs/openapi.yaml) ---

type serverDTO struct {
	ID                  int64           `json:"id"`
	Name                string          `json:"name"`
	Host                string          `json:"host"`
	SSHPort             int             `json:"ssh_port"`
	SSHUser             string          `json:"ssh_user"`
	SSHAuthMethod       string          `json:"ssh_auth_method"`
	XrayConfigPath      string          `json:"xray_config_path,omitempty"`
	XrayServiceName     string          `json:"xray_service_name,omitempty"`
	HysteriaConfigPath  string          `json:"hysteria_config_path,omitempty"`
	HysteriaServiceName string          `json:"hysteria_service_name,omitempty"`
	IP                  string          `json:"ip,omitempty"`
	GeoCountry          string          `json:"geo_country,omitempty"`
	GeoCity             string          `json:"geo_city,omitempty"`
	GeoASN              string          `json:"geo_asn,omitempty"`
	Status              string          `json:"status"`
	ProvisionStatus     string          `json:"provision_status,omitempty"`
	ProvisionError      string          `json:"provision_error,omitempty"`
	Engines             json.RawMessage `json:"engines,omitempty"`
	InboundCount        int             `json:"inbound_count"`
	LastCheckAt         string          `json:"last_check_at,omitempty"`
	LastSyncAt          string          `json:"last_sync_at,omitempty"`
	LastSyncError       string          `json:"last_sync_error,omitempty"`
	CreatedAt           string          `json:"created_at,omitempty"`
	UpdatedAt           string          `json:"updated_at,omitempty"`
}

func toServerDTO(s *store.Server) serverDTO {
	engines := json.RawMessage(s.EnginesJSON)
	if !json.Valid(engines) {
		engines = json.RawMessage("[]")
	}
	return serverDTO{
		ID: s.ID, Name: s.Name, Host: s.Host, SSHPort: s.SSHPort, SSHUser: s.SSHUser,
		SSHAuthMethod: s.SSHAuthMethod, XrayConfigPath: s.XrayConfigPath, XrayServiceName: s.XrayServiceName,
		HysteriaConfigPath: s.HysteriaConfigPath, HysteriaServiceName: s.HysteriaServiceName,
		IP: s.IP, GeoCountry: s.GeoCountry, GeoCity: s.GeoCity, GeoASN: s.GeoASN, Status: s.Status,
		ProvisionStatus: s.ProvisionStatus, ProvisionError: s.ProvisionError, Engines: engines,
		InboundCount: s.InboundCount, LastCheckAt: s.LastCheckAt, LastSyncAt: s.LastSyncAt,
		LastSyncError: s.LastSyncError, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type serverInput struct {
	Name                string `json:"name"`
	Host                string `json:"host"`
	SSHPort             int    `json:"ssh_port"`
	SSHUser             string `json:"ssh_user"`
	SSHAuthMethod       string `json:"ssh_auth_method"`
	SSHSecret           string `json:"ssh_secret"`
	SSHPassphrase       string `json:"ssh_passphrase"`
	XrayConfigPath      string `json:"xray_config_path"`
	XrayServiceName     string `json:"xray_service_name"`
	HysteriaConfigPath  string `json:"hysteria_config_path"`
	HysteriaServiceName string `json:"hysteria_service_name"`
}

func (in serverInput) toServiceInput() servers.Input {
	return servers.Input{
		Name: in.Name, Host: in.Host, SSHPort: in.SSHPort, SSHUser: in.SSHUser,
		SSHAuthMethod: in.SSHAuthMethod, SSHSecret: in.SSHSecret, SSHPassphrase: in.SSHPassphrase,
		XrayConfigPath: in.XrayConfigPath, XrayServiceName: in.XrayServiceName,
		HysteriaConfigPath: in.HysteriaConfigPath, HysteriaServiceName: in.HysteriaServiceName,
	}
}

type serverStatsDTO struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemPercent    float64 `json:"mem_percent"`
	MemUsedMB     float64 `json:"mem_used_mb"`
	MemTotalMB    float64 `json:"mem_total_mb"`
	DiskPercent   float64 `json:"disk_percent"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	CollectedAt   string  `json:"collected_at"`
}

// --- helpers ---

func idParam(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil
}

func (h *serversHandler) audit(r *http.Request, action, targetType string, targetID int64, detail string) {
	id, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, id, action, targetType, targetID, detail)
}

// --- Handlers ---

func (h *serversHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}
	out := make([]serverDTO, 0, len(list))
	for i := range list {
		out = append(out, toServerDTO(&list[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *serversHandler) create(w http.ResponseWriter, r *http.Request) {
	var in serverInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.Name == "" || in.Host == "" {
		writeError(w, http.StatusBadRequest, "name and host are required")
		return
	}
	srv, err := h.svc.Create(r.Context(), in.toServiceInput())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create server")
		return
	}
	h.audit(r, "server.create", "server", srv.ID, `{"name":`+strconv.Quote(srv.Name)+`}`)
	// Auto-provision engines (SPEC §5.1) in the background.
	h.svc.StartProvision(srv.ID)
	if refreshed, err := h.svc.Get(r.Context(), srv.ID); err == nil {
		srv = refreshed
	}
	writeJSON(w, http.StatusCreated, toServerDTO(srv))
}

func (h *serversHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load server")
		return
	}
	writeJSON(w, http.StatusOK, toServerDTO(srv))
}

func (h *serversHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in serverInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	srv, err := h.svc.Update(r.Context(), id, in.toServiceInput())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update server")
		return
	}
	h.audit(r, "server.update", "server", srv.ID, "{}")
	writeJSON(w, http.StatusOK, toServerDTO(srv))
}

func (h *serversHandler) del(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete server")
		return
	}
	h.audit(r, "server.delete", "server", id, "{}")
	w.WriteHeader(http.StatusNoContent)
}

func (h *serversHandler) check(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := h.svc.Check(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "check failed")
		return
	}
	h.audit(r, "server.check", "server", id, `{"status":`+strconv.Quote(srv.Status)+`}`)
	writeJSON(w, http.StatusOK, toServerDTO(srv))
}

func (h *serversHandler) install(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load server")
		return
	}
	h.audit(r, "server.install", "server", id, "{}")
	h.svc.StartProvision(id)
	if refreshed, err := h.svc.Get(r.Context(), id); err == nil {
		srv = refreshed
	}
	writeJSON(w, http.StatusOK, toServerDTO(srv))
}

func (h *serversHandler) restart(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Engine string `json:"engine"`
	}
	_ = decode(r, &body)
	if err := h.svc.RestartEngine(r.Context(), id, body.Engine); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "server not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error(), "engine": body.Engine})
		return
	}
	h.audit(r, "server.restart", "server", id, `{"engine":`+strconv.Quote(body.Engine)+`}`)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "restarted", "engine": body.Engine})
}

func (h *serversHandler) stats(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	st, err := h.svc.Stats(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to collect stats")
		return
	}
	writeJSON(w, http.StatusOK, serverStatsDTO{
		CPUPercent: st.CPUPercent, MemPercent: st.MemPercent, MemUsedMB: st.MemUsedMB,
		MemTotalMB: st.MemTotalMB, DiskPercent: st.DiskPercent, DiskUsedGB: st.DiskUsedGB,
		DiskTotalGB: st.DiskTotalGB, UptimeSeconds: st.UptimeSeconds,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
