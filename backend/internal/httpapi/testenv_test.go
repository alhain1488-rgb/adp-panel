package httpapi

import (
	"crypto/rand"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/inbounds"
	"github.com/adp/panel/internal/logging"
	"github.com/adp/panel/internal/servers"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/ssh/sshtest"
	"github.com/adp/panel/internal/store"
	"github.com/adp/panel/internal/subscription"
	syncpkg "github.com/adp/panel/internal/sync"
)

type testEnv struct {
	router  http.Handler
	svc     *auth.Service
	store   *store.Store
	servers *servers.Service
	clients *clients.Service
	sync    *syncpkg.Service
	runner  *sshtest.MockRunner
	dialer  *sshtest.MockDialer
}

// sampleMetrics is a canned metricsCmd output → ~20.5% CPU, 34.2% mem, 28.3% disk, 10d uptime.
const sampleMetrics = "CPU1 cpu 100 0 50 800 20 0 5 0\n" +
	"CPU2 cpu 110 0 55 860 22 0 6 0\n" +
	"MEM 1400 4096\n" +
	"DISK 17000000 60000000\n" +
	"UPTIME 864000\n"

func defaultSSHHandler(cmd, _ string) (ssh.Result, error) {
	switch {
	case strings.Contains(cmd, "/etc/os-release"):
		return ssh.Result{Stdout: "ubuntu\n"}, nil
	case strings.Contains(cmd, "xray version"):
		return ssh.Result{Stdout: "Xray 1.8.24 (Xray, Penetrates Everything.)\n"}, nil
	case strings.Contains(cmd, "sing-box version"):
		return ssh.Result{Stdout: "sing-box version 1.10.0\n"}, nil
	case strings.Contains(cmd, "is-active"):
		return ssh.Result{Stdout: "active\n"}, nil
	case strings.Contains(cmd, "ifconfig.me"):
		return ssh.Result{Stdout: "203.0.113.7\n"}, nil
	case strings.Contains(cmd, "/proc/stat"):
		return ssh.Result{Stdout: sampleMetrics}, nil
	default:
		return ssh.Result{}, nil
	}
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(database)
	authSvc := auth.NewService(st, cipher, auth.NewTokenManager([]byte("test-secret")))

	runner := &sshtest.MockRunner{Handler: defaultSSHHandler}
	dialer := &sshtest.MockDialer{Runner: runner}
	serversSvc := servers.NewService(st, cipher, dialer, servers.NoopGeo{})
	inboundsSvc := inbounds.NewService(st)
	clientsSvc := clients.NewService(st)
	subscriptionSvc := subscription.NewService(st, clientsSvc)
	syncSvc := syncpkg.NewService(st, serversSvc)

	router := Router(Deps{
		DB:           database,
		Store:        st,
		Auth:         authSvc,
		Servers:      serversSvc,
		Inbounds:     inboundsSvc,
		Clients:      clientsSvc,
		Subscription: subscriptionSvc,
		Sync:         syncSvc,
		Logger:       logging.New(),
		Version:      "test",
		Domain:       "panel.test",
		SubBaseURL:   "http://panel.test",
	})
	return &testEnv{
		router: router, svc: authSvc, store: st, servers: serversSvc,
		clients: clientsSvc, sync: syncSvc, runner: runner, dialer: dialer,
	}
}
