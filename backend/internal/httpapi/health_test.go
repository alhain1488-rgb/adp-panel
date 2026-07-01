package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	env := newTestEnv(t)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Version != "test" {
		t.Errorf("body = %+v", body)
	}
}

func TestReadyz(t *testing.T) {
	env := newTestEnv(t)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want 200", rec.Code)
	}
}
