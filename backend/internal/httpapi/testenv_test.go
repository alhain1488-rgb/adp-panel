package httpapi

import (
	"crypto/rand"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/logging"
	"github.com/adp/panel/internal/store"
)

type testEnv struct {
	router http.Handler
	svc    *auth.Service
	store  *store.Store
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
	svc := auth.NewService(st, cipher, auth.NewTokenManager([]byte("test-secret")))

	router := Router(Deps{
		DB:      database,
		Store:   st,
		Auth:    svc,
		Logger:  logging.New(),
		Version: "test",
	})
	return &testEnv{router: router, svc: svc, store: st}
}
