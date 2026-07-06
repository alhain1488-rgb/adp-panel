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
	"github.com/adp/panel/internal/backup"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/inbounds"
	"github.com/adp/panel/internal/mail"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/subscription"
	syncpkg "github.com/adp/panel/internal/sync"
)

// Deps are the dependencies the HTTP layer needs.
type Deps struct {
	DB           *sql.DB
	Store        *store.Store
	Auth         *auth.Service
	Servers      *servers.Service
	Inbounds     *inbounds.Service
	Clients      *clients.Service
	Subscription *subscription.Service
	Sync         *syncpkg.Service
	Backup       *backup.Service
	Telegram     *backup.Telegram
	EmailBackup  *backup.Email
	Mail         *mail.Mailer
	Logger       *slog.Logger
	Version      string
	Domain       string
	SubBaseURL   string
	Theme        string
	// Restart applies a staged backup restore by restarting the process.
	Restart func()
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

	// API docs (public, dev convenience).
	r.Get("/swagger", health.swaggerUI)
	r.Get("/openapi.yaml", health.openapiYAML)

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

	sh := &serversHandler{svc: d.Servers, sync: d.Sync, store: d.Store}

	// Public subscription endpoint (token-addressed, rate-limited).
	if d.Subscription != nil {
		subH := &subscriptionHandler{svc: d.Subscription}
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(60, time.Minute))
			r.Get("/sub/{token}", subH.get)
		})
	}

	// Public client portal login (name + subscription token, rate-limited).
	if d.Clients != nil {
		ph := &portalHandler{svc: d.Clients, store: d.Store, subBase: d.SubBaseURL}
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(20, time.Minute))
			r.Post("/api/portal/login", ph.login)
		})
	}

	// Protected API.
	r.Group(func(r chi.Router) {
		r.Use(d.Auth.RequireAuth)

		r.Get("/api/logs", ah.listLogs)

		syh := &systemHandler{version: d.Version, domain: d.Domain}
		r.Get("/api/system", syh.get)

		seth := &settingsHandler{store: d.Store, domain: d.Domain, subBaseURL: d.SubBaseURL, defaultTheme: d.Theme}
		r.Get("/api/settings", seth.get)
		r.Put("/api/settings", seth.update)

		if d.Backup != nil {
			bh := &backupHandler{svc: d.Backup, telegram: d.Telegram, email: d.EmailBackup, store: d.Store, logger: d.Logger, restart: d.Restart}
			r.Post("/api/backup/export", bh.export)
			r.Post("/api/backup/import", bh.importBackup)
			if d.Telegram != nil {
				r.Get("/api/backup/telegram", bh.telegramGet)
				r.Put("/api/backup/telegram", bh.telegramPut)
				r.Post("/api/backup/telegram/run", bh.telegramRun)
			}
			if d.EmailBackup != nil {
				r.Get("/api/backup/email", bh.emailGet)
				r.Put("/api/backup/email", bh.emailPut)
				r.Post("/api/backup/email/run", bh.emailRun)
			}
		}

		if d.Mail != nil {
			mh := &mailHandler{mailer: d.Mail, store: d.Store, logger: d.Logger}
			r.Get("/api/mail", mh.get)
			r.Put("/api/mail", mh.put)
			r.Post("/api/mail/test", mh.test)
		}

		ih := &inboundsHandler{svc: d.Inbounds, servers: d.Servers, store: d.Store, sync: d.Sync}

		r.Route("/api/servers", func(r chi.Router) {
			r.Get("/", sh.list)
			r.Post("/", sh.create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", sh.get)
				r.Put("/", sh.update)
				r.Delete("/", sh.del)
				r.Post("/check", sh.check)
				r.Post("/install", sh.install)
				r.Post("/restart-xray", sh.restart)
				r.Post("/sync", sh.syncNode)
				r.Get("/stats", sh.stats)
				r.Get("/inbounds", ih.listByServer)
				r.Post("/inbounds", ih.create)
			})
		})

		r.Route("/api/inbounds/{id}", func(r chi.Router) {
			r.Get("/", ih.get)
			r.Put("/", ih.update)
			r.Delete("/", ih.del)
		})

		ch := &clientsHandler{svc: d.Clients, store: d.Store, subBase: d.SubBaseURL, sync: d.Sync, mailer: d.Mail, telegram: d.Telegram, emailBackup: d.EmailBackup}
		r.Route("/api/clients", func(r chi.Router) {
			r.Get("/", ch.list)
			r.Post("/", ch.create)
			r.Post("/send-configs", ch.sendConfigs)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", ch.get)
				r.Put("/", ch.update)
				r.Delete("/", ch.del)
				r.Post("/enable", ch.enable)
				r.Post("/disable", ch.disable)
				r.Put("/inbounds", ch.setInbounds)
				r.Post("/rotate-token", ch.rotateToken)
				r.Get("/links", ch.links)
				r.Get("/amneziawg", ch.amneziawg)
				r.Get("/qrcode", ch.qrcode)
				r.Get("/config", ch.config)
				r.Post("/email", ch.sendEmail)
				r.Get("/telegram-link", ch.telegramLink)
				r.Post("/telegram-config", ch.telegramConfig)
				r.Post("/telegram-unlink", ch.telegramUnlink)
			})
		})
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
