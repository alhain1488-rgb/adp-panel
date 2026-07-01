package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func (env *testEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func seedAdmin(t *testing.T, env *testEnv) {
	t.Helper()
	if _, err := env.svc.SeedAdmin(context.Background(), "admin", "pw"); err != nil {
		t.Fatal(err)
	}
}

func loginToken(t *testing.T, env *testEnv) string {
	t.Helper()
	rec := env.do(t, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "pw"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	var res loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Need2FA || res.Tokens == nil || res.Tokens.Token == "" {
		t.Fatalf("unexpected login response: %+v", res)
	}
	return res.Tokens.Token
}

func TestLogin_Success(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	rec := env.do(t, http.MethodGet, "/api/auth/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	var me adminDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Username != "admin" || me.TOTPEnabled {
		t.Errorf("me = %+v", me)
	}

	// audit recorded the login
	n, err := env.store.CountAuditLogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("expected an audit entry for login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	rec := env.do(t, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "nope"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMe_NoToken(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	rec := env.do(t, http.MethodGet, "/api/auth/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTOTP_FullCycle(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	// setup
	rec := env.do(t, http.MethodPost, "/api/auth/2fa/setup", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d", rec.Code)
	}
	var setup struct {
		Secret     string `json:"secret"`
		OtpauthURL string `json:"otpauth_url"`
		QR         string `json:"qr"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	if setup.Secret == "" || setup.QR == "" || setup.OtpauthURL == "" {
		t.Fatalf("incomplete setup payload: %+v", setup)
	}

	// enable with a valid code
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec = env.do(t, http.MethodPost, "/api/auth/2fa/enable", token, map[string]string{"code": code})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("enable status = %d, body %s", rec.Code, rec.Body.String())
	}

	// login now requires 2FA
	rec = env.do(t, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "pw"})
	var res loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Need2FA || res.ChallengeID == "" {
		t.Fatalf("expected 2FA challenge, got %+v", res)
	}

	// verify with a fresh code
	code2, _ := totp.GenerateCode(setup.Secret, time.Now())
	rec = env.do(t, http.MethodPost, "/api/auth/2fa/verify", "", map[string]string{"code": code2, "challenge_id": res.ChallengeID})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body %s", rec.Code, rec.Body.String())
	}
	var tokens authTokens
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatal(err)
	}
	if tokens.Token == "" {
		t.Error("expected token after 2FA verify")
	}
}

func TestEnable2FA_BadCode(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	if rec := env.do(t, http.MethodPost, "/api/auth/2fa/setup", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d", rec.Code)
	}
	rec := env.do(t, http.MethodPost, "/api/auth/2fa/enable", token, map[string]string{"code": "000000"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
