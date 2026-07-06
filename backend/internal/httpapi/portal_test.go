package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/adp/panel/internal/clients"
)

func TestPortalLogin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	c, err := env.clients.Create(ctx, clients.Input{Name: "Alice"})
	if err != nil {
		t.Fatal(err)
	}

	// Wrong token and wrong name both give the same generic 401 (no oracle).
	if rec := env.do(t, http.MethodPost, "/api/portal/login", "",
		map[string]string{"name": "Alice", "token": "nope"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d", rec.Code)
	}
	if rec := env.do(t, http.MethodPost, "/api/portal/login", "",
		map[string]string{"name": "Bob", "token": c.SubscriptionToken}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong name = %d", rec.Code)
	}

	// Correct login (name is case-insensitive) returns the subscription bundle.
	rec := env.do(t, http.MethodPost, "/api/portal/login", "",
		map[string]string{"name": "alice", "token": c.SubscriptionToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "http://panel.test/sub/"+c.SubscriptionToken) {
		t.Fatalf("bundle missing subscription url: %s", rec.Body.String())
	}

	// A full subscription URL is accepted in place of the bare token.
	if rec := env.do(t, http.MethodPost, "/api/portal/login", "",
		map[string]string{"name": "Alice", "token": "http://panel.test/sub/" + c.SubscriptionToken}); rec.Code != http.StatusOK {
		t.Fatalf("full-url token = %d", rec.Code)
	}

	// A disabled client can authenticate but is refused with a distinct status.
	if _, err := env.store.SetClientEnabled(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	if rec := env.do(t, http.MethodPost, "/api/portal/login", "",
		map[string]string{"name": "Alice", "token": c.SubscriptionToken}); rec.Code != http.StatusForbidden {
		t.Fatalf("disabled = %d", rec.Code)
	}
}

func TestPortalToken(t *testing.T) {
	cases := map[string]string{
		"abc":                              "abc",
		"  abc  ":                          "abc",
		"http://p/sub/tok123":              "tok123",
		"https://p.example/sub/tok123/":    "tok123",
		"https://p.example/sub/tok123?x=1": "tok123",
		"https://p.example/sub/tok123#f":   "tok123",
	}
	for in, want := range cases {
		if got := portalToken(in); got != want {
			t.Errorf("portalToken(%q) = %q, want %q", in, got, want)
		}
	}
}
