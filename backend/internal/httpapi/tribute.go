package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/tribute"
)

// maxWebhookBody caps what we read from an unauthenticated endpoint.
const maxWebhookBody = 1 << 20

// tributeHandler serves the public Tribute webhook plus the operator settings.
type tributeHandler struct {
	svc   *tribute.Service
	store *store.Store
}

// webhook is the only unauthenticated write endpoint in the panel. Its gate is
// the HMAC signature over the raw body, keyed with the Tribute API key, so the
// body must be read verbatim — re-encoding it would change the digest.
//
// Anything we cannot act on still answers 200: Tribute retries for ~24 hours,
// and re-delivering an event we will never accept (an unmapped product, a payer
// we don't know) only produces noise. Genuine failures on our side return 500 so
// the retry is useful.
func (h *tributeHandler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read body")
		return
	}
	err = h.svc.HandleWebhook(r.Context(), body, r.Header.Get(tribute.SignatureHeader))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, tribute.ErrBadSignature), errors.Is(err, tribute.ErrNotConfigured):
		// Never hint at which half was wrong.
		writeError(w, http.StatusUnauthorized, "invalid signature")
	case errors.Is(err, tribute.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "Tribute payments are disabled")
	case errors.Is(err, tribute.ErrIgnored),
		errors.Is(err, tribute.ErrUnknownProduct),
		errors.Is(err, tribute.ErrUnknownClient):
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	default:
		writeError(w, http.StatusInternalServerError, "could not apply the purchase")
	}
}

func (h *tributeHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Tribute settings")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *tributeHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		tribute.Settings
		// APIKey is write-only: blank keeps whatever is stored.
		APIKey string `json:"api_key"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SetSettings(r.Context(), in.Settings, in.APIKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save Tribute settings")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "billing.tribute", "billing", 0, "{}")
	h.getSettings(w, r)
}
