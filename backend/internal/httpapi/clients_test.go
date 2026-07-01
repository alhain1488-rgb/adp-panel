package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// createInbound posts an inbound on a server and returns its id.
func createInbound(t *testing.T, env *testEnv, token string, serverID int64, body map[string]any) int64 {
	t.Helper()
	rec := env.do(t, http.MethodPost, "/api/servers/"+itoa(serverID)+"/inbounds", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create inbound status = %d, body %s", rec.Code, rec.Body.String())
	}
	var dto inboundDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	return dto.ID
}

// TestClientsSubscription_EndToEnd covers the Phase 6 acceptance path: a client
// granted inbounds across both engines yields a non-empty subscription with the
// right URIs; disabling the client empties it; rotating the token invalidates
// the old one.
func TestClientsSubscription_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	srv := createServer(t, env, token, "de-1")

	vlessID := createInbound(t, env, token, srv.ID, map[string]any{
		"tag": "vless-tcp", "protocol": "vless", "port": 443,
		"settings":        map[string]any{"decryption": "none"},
		"stream_settings": map[string]any{"network": "tcp", "security": "none"},
	})
	hyID := createInbound(t, env, token, srv.ID, map[string]any{
		"tag": "hy2", "protocol": "hysteria2", "port": 36712,
		"settings": map[string]any{"up": "100 mbps", "down": "200 mbps"},
	})

	// Create a client.
	rec := env.do(t, http.MethodPost, "/api/clients", token, map[string]any{"name": "alice"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create client = %d, body %s", rec.Code, rec.Body.String())
	}
	var client clientDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &client)
	if client.UUID == "" || client.SubscriptionToken == "" {
		t.Fatalf("credentials not generated: %+v", client)
	}
	if client.SubscriptionURL != "http://panel.test/sub/"+client.SubscriptionToken {
		t.Errorf("subscription_url = %q", client.SubscriptionURL)
	}

	// Grant both inbounds.
	rec = env.do(t, http.MethodPut, "/api/clients/"+itoa(client.ID)+"/inbounds", token,
		map[string]any{"inbound_ids": []int64{vlessID, hyID}})
	if rec.Code != http.StatusOK {
		t.Fatalf("grant = %d, body %s", rec.Code, rec.Body.String())
	}
	var granted clientDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &granted)
	if len(granted.InboundIDs) != 2 || len(granted.Grants) != 2 {
		t.Fatalf("grants = %+v", granted)
	}

	// Public subscription: base64 of two URIs (vless + hysteria2).
	uris := fetchSub(t, env, client.SubscriptionToken, http.StatusOK)
	if len(uris) != 2 {
		t.Fatalf("subscription uris = %d (%v), want 2", len(uris), uris)
	}
	var hasVless, hasHy bool
	for _, u := range uris {
		if strings.HasPrefix(u, "vless://") {
			hasVless = true
		}
		if strings.HasPrefix(u, "hysteria2://") {
			hasHy = true
		}
	}
	if !hasVless || !hasHy {
		t.Errorf("missing a scheme: %v", uris)
	}

	// Links endpoint mirrors the subscription content.
	rec = env.do(t, http.MethodGet, "/api/clients/"+itoa(client.ID)+"/links", token, nil)
	var links []clientLinkDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &links)
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2", len(links))
	}

	// Disabling the client empties the subscription.
	rec = env.do(t, http.MethodPost, "/api/clients/"+itoa(client.ID)+"/disable", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable = %d", rec.Code)
	}
	if uris := fetchSub(t, env, client.SubscriptionToken, http.StatusOK); len(uris) != 0 {
		t.Errorf("disabled client subscription not empty: %v", uris)
	}

	// Re-enable, then rotate the token: the old token stops working.
	_ = env.do(t, http.MethodPost, "/api/clients/"+itoa(client.ID)+"/enable", token, nil)
	rec = env.do(t, http.MethodPost, "/api/clients/"+itoa(client.ID)+"/rotate-token", token, nil)
	var rotated clientDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &rotated)
	if rotated.SubscriptionToken == client.SubscriptionToken {
		t.Error("token not rotated")
	}
	fetchSub(t, env, client.SubscriptionToken, http.StatusNotFound) // old token → 404
	if uris := fetchSub(t, env, rotated.SubscriptionToken, http.StatusOK); len(uris) != 2 {
		t.Errorf("new token subscription = %v", uris)
	}
}

// fetchSub GETs /sub/{token}, asserts the status, and returns the decoded URIs.
func fetchSub(t *testing.T, env *testEnv, token string, wantStatus int) []string {
	t.Helper()
	rec := env.do(t, http.MethodGet, "/sub/"+token, "", nil)
	if rec.Code != wantStatus {
		t.Fatalf("/sub status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body.String())
	}
	if wantStatus != http.StatusOK {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(rec.Body.String())
	if err != nil {
		t.Fatalf("subscription not base64: %v", err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func TestClients_SyncEndpoint(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	srv := createServer(t, env, token, "de-1")
	createInbound(t, env, token, srv.ID, map[string]any{
		"tag": "vless-tcp", "protocol": "vless", "port": 443,
		"settings": map[string]any{"decryption": "none"},
	})
	// Mark the node installed so sync will push (mock SSH accepts all commands).
	if err := env.store.SetProvision(context.Background(), srv.ID, "installed", "", ""); err != nil {
		t.Fatal(err)
	}

	rec := env.do(t, http.MethodPost, "/api/servers/"+itoa(srv.ID)+"/sync", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d, body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK      bool `json:"ok"`
		Engines []struct {
			Engine  string `json:"engine"`
			Changed bool   `json:"changed"`
		} `json:"engines"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.OK || len(resp.Engines) != 1 || resp.Engines[0].Engine != "xray" {
		t.Fatalf("sync response = %s", rec.Body.String())
	}
	if !env.runner.Ran("systemctl restart 'xray'") {
		t.Error("expected xray restart during sync")
	}
}
