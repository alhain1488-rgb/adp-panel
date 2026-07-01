package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	skipqr "github.com/skip2/go-qrcode"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/store"
)

type clientsHandler struct {
	svc     *clients.Service
	store   *store.Store
	subBase string
}

// --- DTOs (mirror docs/openapi.yaml) ---

type clientDTO struct {
	ID                int64            `json:"id"`
	Name              string           `json:"name"`
	UUID              string           `json:"uuid"`
	Password          string           `json:"password,omitempty"`
	SubscriptionToken string           `json:"subscription_token"`
	SubscriptionURL   string           `json:"subscription_url,omitempty"`
	Enabled           bool             `json:"enabled"`
	Remark            string           `json:"remark,omitempty"`
	InboundIDs        []int64          `json:"inbound_ids"`
	Grants            []clientGrantDTO `json:"grants,omitempty"`
	CreatedAt         string           `json:"created_at,omitempty"`
	UpdatedAt         string           `json:"updated_at,omitempty"`
}

type clientGrantDTO struct {
	InboundID  int64  `json:"inbound_id"`
	ServerID   int64  `json:"server_id"`
	ServerName string `json:"server_name,omitempty"`
	InboundTag string `json:"inbound_tag,omitempty"`
	Protocol   string `json:"protocol"`
	Enabled    bool   `json:"enabled"`
}

type clientLinkDTO struct {
	InboundID  int64  `json:"inbound_id"`
	ServerName string `json:"server_name,omitempty"`
	Protocol   string `json:"protocol"`
	Remark     string `json:"remark,omitempty"`
	URI        string `json:"uri"`
}

type clientInput struct {
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

type clientInboundsInput struct {
	InboundIDs []int64 `json:"inbound_ids"`
}

func (h *clientsHandler) subURL(token string) string {
	if h.subBase == "" {
		return ""
	}
	return strings.TrimRight(h.subBase, "/") + "/sub/" + token
}

// toDTO maps a client, loading its grant ids (and optionally full grants).
func (h *clientsHandler) toDTO(ctx context.Context, c *store.Client, withGrants bool) clientDTO {
	ids, _ := h.svc.GrantedInboundIDs(ctx, c.ID)
	if ids == nil {
		ids = []int64{}
	}
	dto := clientDTO{
		ID: c.ID, Name: c.Name, UUID: c.UUID, Password: c.Password,
		SubscriptionToken: c.SubscriptionToken, SubscriptionURL: h.subURL(c.SubscriptionToken),
		Enabled: c.Enabled, Remark: c.Remark, InboundIDs: ids,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if withGrants {
		grants, _ := h.svc.Grants(ctx, c.ID)
		dto.Grants = make([]clientGrantDTO, 0, len(grants))
		for _, g := range grants {
			dto.Grants = append(dto.Grants, clientGrantDTO{
				InboundID: g.InboundID, ServerID: g.ServerID, ServerName: g.ServerName,
				InboundTag: g.InboundTag, Protocol: g.Protocol, Enabled: g.GrantEnabled && g.InboundOn,
			})
		}
	}
	return dto
}

func (h *clientsHandler) audit(r *http.Request, action string, targetID int64, detail string) {
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, action, "client", targetID, detail)
}

// --- Handlers ---

func (h *clientsHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}
	out := make([]clientDTO, 0, len(list))
	for i := range list {
		out = append(out, h.toDTO(r.Context(), &list[i], false))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *clientsHandler) create(w http.ResponseWriter, r *http.Request) {
	var in clientInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	c, err := h.svc.Create(r.Context(), clients.Input{Name: in.Name, Remark: in.Remark})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create client")
		return
	}
	h.audit(r, "client.create", c.ID, `{"name":`+strconv.Quote(c.Name)+`}`)
	writeJSON(w, http.StatusCreated, h.toDTO(r.Context(), c, false))
}

func (h *clientsHandler) get(w http.ResponseWriter, r *http.Request) {
	c, ok := h.load(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in clientInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.Update(r.Context(), id, clients.Input{Name: in.Name, Remark: in.Remark})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update client")
		return
	}
	h.audit(r, "client.update", c.ID, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) del(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete client")
		return
	}
	h.audit(r, "client.delete", id, "{}")
	w.WriteHeader(http.StatusNoContent)
}

func (h *clientsHandler) enable(w http.ResponseWriter, r *http.Request) { h.setEnabled(w, r, true) }
func (h *clientsHandler) disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *clientsHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.SetEnabled(r.Context(), id, enabled)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update client")
		return
	}
	action := "client.disable"
	if enabled {
		action = "client.enable"
	}
	h.audit(r, action, id, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) setInbounds(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in clientInboundsInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.SetInbounds(r.Context(), id, in.InboundIDs)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update grants")
		return
	}
	h.audit(r, "client.grants", id, `{"count":`+strconv.Itoa(len(in.InboundIDs))+`}`)
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) rotateToken(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := h.svc.RotateToken(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate token")
		return
	}
	h.audit(r, "client.rotate_token", id, "{}")
	writeJSON(w, http.StatusOK, h.toDTO(r.Context(), c, true))
}

func (h *clientsHandler) links(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	links, err := h.svc.Links(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build links")
		return
	}
	out := make([]clientLinkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, clientLinkDTO{
			InboundID: l.InboundID, ServerName: l.ServerName, Protocol: l.Protocol,
			Remark: l.Remark, URI: l.URI,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *clientsHandler) qrcode(w http.ResponseWriter, r *http.Request) {
	c, ok := h.load(w, r)
	if !ok {
		return
	}
	target := r.URL.Query().Get("link")
	if target == "" {
		target = h.subURL(c.SubscriptionToken)
	}
	if target == "" {
		writeError(w, http.StatusBadRequest, "no link to encode")
		return
	}
	png, err := skipqr.Encode(target, skipqr.Medium, 512)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render qr")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *clientsHandler) config(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	links, err := h.svc.Links(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build config")
		return
	}
	uris := make([]string, 0, len(links))
	for _, l := range links {
		uris = append(uris, l.URI)
	}
	body := strings.Join(uris, "\n")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="subscription.txt"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// load resolves the {id} param to a client or writes the error response.
func (h *clientsHandler) load(w http.ResponseWriter, r *http.Request) (*store.Client, bool) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	c, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return nil, false
	}
	return c, true
}
