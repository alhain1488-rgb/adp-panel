// Package httpapi wires the HTTP router and server. Phase 2 exposes health
// endpoints; later phases mount the REST API and subscription handlers.
package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Deps are the dependencies the HTTP layer needs.
type Deps struct {
	DB      *sql.DB
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

	h := &healthHandler{db: d.DB, version: d.Version}
	r.Get("/healthz", h.healthz)
	r.Get("/readyz", h.readyz)

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
