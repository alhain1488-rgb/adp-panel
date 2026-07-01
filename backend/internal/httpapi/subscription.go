package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/subscription"
)

type subscriptionHandler struct {
	svc *subscription.Service
}

// get serves the public, token-addressed subscription as a base64 body. This is
// the only unauthenticated endpoint (SPEC invariant).
func (h *subscriptionHandler) get(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	body, err := h.svc.Build(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build subscription")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
