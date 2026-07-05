package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/mail"
	"github.com/adp/panel/internal/store"
)

// mailHandler manages the SMTP configuration used for e-mail delivery (backups
// and client subscriptions).
type mailHandler struct {
	mailer *mail.Mailer
	store  *store.Store
	logger *slog.Logger
}

// get returns the SMTP config without the password.
func (h *mailHandler) get(w http.ResponseWriter, r *http.Request) {
	status, err := h.mailer.Status(r.Context())
	if err != nil {
		h.logger.Error("read smtp config failed", "err", err)
		writeError(w, http.StatusInternalServerError, "could not read config")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// put updates the SMTP config.
func (h *mailHandler) put(w http.ResponseWriter, r *http.Request) {
	var in mail.Input
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.mailer.SetConfig(r.Context(), in); err != nil {
		h.logger.Error("save smtp config failed", "err", err)
		writeError(w, http.StatusInternalServerError, "could not save config")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "mail.config", "mail", 0, "")

	status, err := h.mailer.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read config")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

type mailTestInput struct {
	To string `json:"to"`
}

// test sends a short test message to verify the SMTP settings work.
func (h *mailHandler) test(w http.ResponseWriter, r *http.Request) {
	var in mailTestInput
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.To == "" {
		writeError(w, http.StatusBadRequest, "a recipient address is required")
		return
	}
	body := "This is a test message from your ADP panel. If you received it, SMTP is working.\n"
	if err := h.mailer.Send(r.Context(), []string{in.To}, "ADP panel — SMTP test", body, nil); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "mail.test", "mail", 0, "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
