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
	"github.com/adp/panel/internal/backup"
	"github.com/adp/panel/internal/billing"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/config"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/httpapi"
	"github.com/adp/panel/internal/inbounds"
	"github.com/adp/panel/internal/logging"
	"github.com/adp/panel/internal/mail"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/subscription"
	syncpkg "github.com/adp/panel/internal/sync"
)

// version is the backend build version; kept in sync with the frontend APP_VERSION.
const version = "0.9.14.0"

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
	syncSvc := syncpkg.NewService(st, serversSvc, clientsSvc)
	backupSvc := backup.NewService(database, cfg.DBPath, cfg.EncryptionKey, version)
	billingSvc := billing.NewService(st, syncSvc, logger)
	telegramSvc := backup.NewTelegram(backupSvc, st, cipher, logger)
	telegramSvc.SetBilling(billingSvc)
	billingSvc.SetStarRefunder(telegramSvc)
	telegramSvc.SetSignup(clientsSvc)
	mailer := mail.NewMailer(st, cipher)
	emailBackupSvc := backup.NewEmail(backupSvc, mailer, st, cipher, logger)

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
		Backup:       backupSvc,
		Telegram:     telegramSvc,
		EmailBackup:  emailBackupSvc,
		Billing:      billingSvc,
		Mail:         mailer,
		Logger:       logger,
		Version:      version,
		Domain:       cfg.Domain,
		SubBaseURL:   cfg.SubBaseURL,
		// Restore is applied on the next boot; trigger a graceful restart so the
		// staged database is swapped in (works under Docker's restart policy).
		Restart: func() {
			logger.Info("applying backup restore, restarting")
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		},
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

	// Scheduled backups run until shutdown.
	go telegramSvc.RunScheduler(ctx)
	go emailBackupSvc.RunScheduler(ctx)

	// Billing reconciler auto-suspends managed clients whose subscription lapsed.
	go billingSvc.RunReconciler(ctx)

	// Poll Telegram for clients opening their "/start" deep link; deliver their
	// config the moment they link, so the operator needn't do anything.
	sendClientTG := func(ctx context.Context, c *store.Client) {
		url, qr, err := clientsSvc.SubscriptionConfig(ctx, c.ID, cfg.SubBaseURL)
		if err != nil {
			logger.Warn("telegram: build client config failed", "client", c.ID, "err", err)
			return
		}
		if err := telegramSvc.SendClientConfig(ctx, c.TelegramChatID, c.Name, url, qr); err != nil {
			logger.Warn("telegram: send client config failed", "client", c.ID, "err", err)
		}
	}
	go telegramSvc.RunLinkPoller(ctx, sendClientTG)

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
