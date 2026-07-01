package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/store"
)

type authHandler struct {
	svc   *auth.Service
	store *store.Store
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// clientIP returns the request IP without the port.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (h *authHandler) audit(ctx context.Context, r *http.Request, adminID int64, action, targetType string, targetID int64, detail string) {
	_ = h.store.InsertAuditLog(ctx, store.AuditLog{
		AdminID:    adminID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		DetailJSON: detail,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
}

// --- DTOs (mirror docs/openapi.yaml) ---

type authTokens struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type loginResponse struct {
	Need2FA     bool        `json:"need_2fa"`
	Tokens      *authTokens `json:"tokens,omitempty"`
	ChallengeID string      `json:"challenge_id,omitempty"`
}

type adminDTO struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	TOTPEnabled bool   `json:"totp_enabled"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func toAdminDTO(a *store.Admin) adminDTO {
	return adminDTO{ID: a.ID, Username: a.Username, TOTPEnabled: a.TOTPEnabled, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}

// --- Handlers ---

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	res, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if res.Need2FA {
		writeJSON(w, http.StatusOK, loginResponse{Need2FA: true, ChallengeID: res.ChallengeID})
		return
	}
	h.audit(r.Context(), r, res.AdminID, "login", "admin", res.AdminID, `{"method":"password"}`)
	writeJSON(w, http.StatusOK, loginResponse{
		Need2FA: false,
		Tokens:  &authTokens{Token: res.Token, ExpiresAt: res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")},
	})
}

func (h *authHandler) verify2FA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		ChallengeID string `json:"challenge_id"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	res, err := h.svc.VerifyTOTP(r.Context(), req.ChallengeID, req.Code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	h.audit(r.Context(), r, res.AdminID, "login", "admin", res.AdminID, `{"method":"password+totp"}`)
	writeJSON(w, http.StatusOK, authTokens{
		Token:     res.Token,
		ExpiresAt: res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *authHandler) setup2FA(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.AdminIDFrom(r.Context())
	setup, err := h.svc.SetupTOTP(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start 2FA setup")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"otpauth_url": setup.OtpauthURL,
		"qr":          setup.QR,
		"secret":      setup.Secret,
	})
}

func (h *authHandler) enable2FA(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.AdminIDFrom(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.EnableTOTP(r.Context(), id, req.Code); err != nil {
		if errors.Is(err, auth.ErrInvalidCode) || errors.Is(err, auth.ErrTOTPNotSetup) {
			writeError(w, http.StatusUnauthorized, "invalid code")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to enable 2FA")
		return
	}
	h.audit(r.Context(), r, id, "2fa.enable", "admin", id, "{}")
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	// Stateless JWT: the client discards the token. Record the action if known.
	if id, ok := auth.AdminIDFrom(r.Context()); ok {
		h.audit(r.Context(), r, id, "logout", "admin", id, "{}")
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.AdminIDFrom(r.Context())
	admin, err := h.svc.Admin(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, toAdminDTO(admin))
}
