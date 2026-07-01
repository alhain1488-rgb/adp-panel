package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListLogs(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env) // produces a "login" audit entry

	rec := env.do(t, http.MethodGet, "/api/logs?page=1&page_size=10", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d", rec.Code)
	}
	var page auditLogPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total < 1 || len(page.Items) < 1 {
		t.Fatalf("expected at least one log, got %+v", page)
	}
	if page.Items[0].Action != "login" {
		t.Errorf("newest action = %q, want login", page.Items[0].Action)
	}
	if len(page.Items[0].Detail) == 0 {
		t.Error("detail should be present JSON")
	}
}

func TestListLogs_RequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/logs", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
