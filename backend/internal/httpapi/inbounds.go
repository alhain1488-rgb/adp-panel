package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/inbounds"
	"github.com/adp/panel/internal/protocols"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/store"
)

type inboundsHandler struct {
	svc     *inbounds.Service
	servers *servers.Service
	store   *store.Store
}

type inboundDTO struct {
	ID             int64           `json:"id"`
	ServerID       int64           `json:"server_id"`
	Tag            string          `json:"tag"`
	Protocol       string          `json:"protocol"`
	Engine         string          `json:"engine,omitempty"`
	Listen         string          `json:"listen,omitempty"`
	Port           int             `json:"port"`
	Settings       json.RawMessage `json:"settings,omitempty"`
	StreamSettings json.RawMessage `json:"stream_settings,omitempty"`
	Sniffing       json.RawMessage `json:"sniffing,omitempty"`
	Remark         string          `json:"remark,omitempty"`
	Enabled        bool            `json:"enabled"`
	ClientCount    int             `json:"client_count"`
	CreatedAt      string          `json:"created_at,omitempty"`
	UpdatedAt      string          `json:"updated_at,omitempty"`
}

func rawJSON(s string) json.RawMessage {
	if !json.Valid([]byte(s)) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(s)
}

func toInboundDTO(in *store.Inbound) inboundDTO {
	engine, _ := protocols.EngineOf(in.Protocol)
	return inboundDTO{
		ID: in.ID, ServerID: in.ServerID, Tag: in.Tag, Protocol: in.Protocol, Engine: string(engine),
		Listen: in.Listen, Port: in.Port, Settings: rawJSON(in.SettingsJSON),
		StreamSettings: rawJSON(in.StreamSettingsJSON), Sniffing: rawJSON(in.SniffingJSON),
		Remark: in.Remark, Enabled: in.Enabled, ClientCount: in.ClientCount,
		CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt,
	}
}

type inboundInput struct {
	Tag            string         `json:"tag"`
	Protocol       string         `json:"protocol"`
	Listen         string         `json:"listen"`
	Port           int            `json:"port"`
	Settings       map[string]any `json:"settings"`
	StreamSettings map[string]any `json:"stream_settings"`
	Sniffing       map[string]any `json:"sniffing"`
	Remark         string         `json:"remark"`
	Enabled        *bool          `json:"enabled"`
}

func (in inboundInput) toServiceInput() inbounds.Input {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return inbounds.Input{
		Tag: in.Tag, Protocol: in.Protocol, Listen: in.Listen, Port: in.Port,
		Settings: in.Settings, StreamSettings: in.StreamSettings, Sniffing: in.Sniffing,
		Remark: in.Remark, Enabled: enabled,
	}
}

func (h *inboundsHandler) audit(r *http.Request, action string, targetID int64, detail string) {
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, action, "inbound", targetID, detail)
}

func (h *inboundsHandler) listByServer(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.servers.Get(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	list, err := h.svc.ListByServer(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list inbounds")
		return
	}
	out := make([]inboundDTO, 0, len(list))
	for i := range list {
		out = append(out, toInboundDTO(&list[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *inboundsHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.servers.Get(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	var in inboundInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.svc.Create(r.Context(), id, in.toServiceInput())
	if errors.Is(err, inbounds.ErrUnknownProtocol) {
		writeError(w, http.StatusBadRequest, "unknown protocol")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create inbound")
		return
	}
	h.audit(r, "inbound.create", created.ID, `{"tag":`+strconv.Quote(created.Tag)+`,"server_id":`+strconv.FormatInt(id, 10)+`}`)
	writeJSON(w, http.StatusCreated, toInboundDTO(created))
}

func (h *inboundsHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	in, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "inbound not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load inbound")
		return
	}
	writeJSON(w, http.StatusOK, toInboundDTO(in))
}

func (h *inboundsHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in inboundInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.svc.Update(r.Context(), id, in.toServiceInput())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "inbound not found")
		return
	}
	if errors.Is(err, inbounds.ErrUnknownProtocol) {
		writeError(w, http.StatusBadRequest, "unknown protocol")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update inbound")
		return
	}
	h.audit(r, "inbound.update", updated.ID, "{}")
	writeJSON(w, http.StatusOK, toInboundDTO(updated))
}

func (h *inboundsHandler) del(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "inbound not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete inbound")
		return
	}
	h.audit(r, "inbound.delete", id, "{}")
	w.WriteHeader(http.StatusNoContent)
}
