// Command panel is the Xray panel backend entrypoint.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/config"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/httpapi"
	"github.com/adp/panel/internal/inbounds"
	"github.com/adp/panel/internal/logging"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/subscription"
	syncpkg "github.com/adp/panel/internal/sync"
)

// version is the backend build version; kept in sync with the frontend APP_VERSION.
const version = "0.8.0.1"

func main() {
	logger := logging.New()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("configuration error", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("database error", "err", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("database ready", "path", cfg.DBPath)

	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		logger.Error("crypto init error", "err", err)
		os.Exit(1)
	}

	st := store.New(database)
	authSvc := auth.NewService(st, cipher, auth.NewTokenManager(cfg.JWTSecret))
	serversSvc := servers.NewService(st, cipher, ssh.NewDialer(), servers.NewIPAPIGeo())
	inboundsSvc := inbounds.NewService(st)
	clientsSvc := clients.NewService(st)
	subscriptionSvc := subscription.NewService(st, clientsSvc)
	syncSvc := syncpkg.NewService(st, serversSvc)

	seeded, err := authSvc.SeedAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword)
	if err != nil {
		logger.Error("admin seed error", "err", err)
		os.Exit(1)
	}
	if seeded {
		logger.Info("initial admin created", "username", cfg.AdminUsername)
	}

	handler := httpapi.Router(httpapi.Deps{
		DB:           database,
		Store:        st,
		Auth:         authSvc,
		Servers:      serversSvc,
		Inbounds:     inboundsSvc,
		Clients:      clientsSvc,
		Subscription: subscriptionSvc,
		Sync:         syncSvc,
		Logger:       logger,
		Version:      version,
		Domain:       cfg.Domain,
		SubBaseURL:   cfg.SubBaseURL,
	})
	srv := httpapi.NewServer(cfg.HTTPAddr, handler)

	// Run the server until a termination signal arrives.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		logger.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("stopped")
}
