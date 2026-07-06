package httpapi

import (
	"net/http"
	"strings"

	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/store"
)

// portalHandler serves the public client portal: a subscriber logs in with their
// name + subscription token and sees their own (read-only) subscription. This is
// the only client-facing authenticated surface besides /sub/{token}.
type portalHandler struct {
	svc     *clients.Service
	store   *store.Store
	subBase string
}

type portalLoginInput struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type portalDTO struct {
	Name            string                    `json:"name"`
	SubscriptionURL string                    `json:"subscription_url"`
	Links           []clientLinkDTO           `json:"links"`
	AmneziaWG       []clients.AWGClientConfig `json:"amneziawg,omitempty"`
}

// portalInvalid is a deliberately generic message so the endpoint isn't an oracle
// for valid names or tokens.
const portalInvalid = "Неверное имя или токен (Invalid name or token)"

// portalToken extracts the subscription token from a raw token or a full
// subscription URL (".../sub/<token>[/?#...]").
func portalToken(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "/sub/"); i >= 0 {
		s = s[i+len("/sub/"):]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (h *portalHandler) subURL(token string) string {
	if h.subBase == "" || token == "" {
		return ""
	}
	return strings.TrimRight(h.subBase, "/") + "/sub/" + token
}

// login authenticates a client by name + subscription token and returns their
// subscription bundle. The token is the real secret; the name is a second factor.
func (h *portalHandler) login(w http.ResponseWriter, r *http.Request) {
	var in portalLoginInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token := portalToken(in.Token)
	name := strings.TrimSpace(in.Name)
	if token == "" || name == "" {
		writeError(w, http.StatusUnauthorized, portalInvalid)
		return
	}
	c, err := h.store.GetClientByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, portalInvalid)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(c.Name), name) {
		writeError(w, http.StatusUnauthorized, portalInvalid)
		return
	}
	if !c.Enabled {
		writeError(w, http.StatusForbidden,
			"Ваш доступ отключён администратором (Your access has been disabled by the administrator.)")
		return
	}

	links, _ := h.svc.Links(r.Context(), c.ID)
	linkDTOs := make([]clientLinkDTO, 0, len(links))
	for _, l := range links {
		linkDTOs = append(linkDTOs, clientLinkDTO{
			InboundID: l.InboundID, ServerName: l.ServerName, Protocol: l.Protocol,
			Remark: l.Remark, URI: l.URI,
		})
	}
	awg, _ := h.svc.AmneziaWGConfigs(r.Context(), c.ID)

	writeJSON(w, http.StatusOK, portalDTO{
		Name:            c.Name,
		SubscriptionURL: h.subURL(c.SubscriptionToken),
		Links:           linkDTOs,
		AmneziaWG:       awg,
	})
}
