// Package httpapi wires the HTTP router and server. Phase 2 exposed health
// endpoints; Phase 3 adds authentication (login, 2FA) and the audit trail.
package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/store"
)

// Deps are the dependencies the HTTP layer needs.
type Deps struct {
	DB      *sql.DB
	Store   *store.Store
	Auth    *auth.Service
	Logger  *slog.Logger
	Version string
}

// Router builds the chi router with middleware and routes mounted.
func Router(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	health := &healthHandler{db: d.DB, version: d.Version}
	r.Get("/healthz", health.healthz)
	r.Get("/readyz", health.readyz)

	ah := &authHandler{svc: d.Auth, store: d.Store}

	r.Route("/api/auth", func(r chi.Router) {
		// Rate-limit the credential endpoints per IP.
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(15, time.Minute))
			r.Post("/login", ah.login)
			r.Post("/2fa/verify", ah.verify2FA)
		})
		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(d.Auth.RequireAuth)
			r.Post("/2fa/setup", ah.setup2FA)
			r.Post("/2fa/enable", ah.enable2FA)
			r.Post("/logout", ah.logout)
			r.Get("/me", ah.me)
		})
	})

	// Audit log (read) — protected.
	r.Group(func(r chi.Router) {
		r.Use(d.Auth.RequireAuth)
		r.Get("/api/logs", ah.listLogs)
	})

	return r
}

// NewServer returns an http.Server configured with sane timeouts.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
