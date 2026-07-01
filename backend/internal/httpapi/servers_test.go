package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func createServer(t *testing.T, env *testEnv, token, name string) serverDTO {
	t.Helper()
	rec := env.do(t, http.MethodPost, "/api/servers", token, map[string]any{
		"name": name, "host": "node.example.com", "ssh_user": "root",
		"ssh_auth_method": "password", "ssh_secret": "pw",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	var dto serverDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	return dto
}

func TestServers_CRUD(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	srv := createServer(t, env, token, "de-frankfurt-1")
	if srv.Name != "de-frankfurt-1" || srv.Host != "node.example.com" {
		t.Fatalf("created = %+v", srv)
	}

	// list
	rec := env.do(t, http.MethodGet, "/api/servers", token, nil)
	var list []serverDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	// update
	rec = env.do(t, http.MethodPut, "/api/servers/"+itoa(srv.ID), token, map[string]any{
		"name": "renamed", "host": "node.example.com", "ssh_user": "root", "ssh_auth_method": "password",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d", rec.Code)
	}
	var updated serverDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != "renamed" {
		t.Errorf("name = %q, want renamed", updated.Name)
	}

	// delete
	rec = env.do(t, http.MethodDelete, "/api/servers/"+itoa(srv.ID), token, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec = env.do(t, http.MethodGet, "/api/servers/"+itoa(srv.ID), token, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}

func TestServers_Check(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	srv := createServer(t, env, token, "s1")

	rec := env.do(t, http.MethodPost, "/api/servers/"+itoa(srv.ID)+"/check", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("check status = %d", rec.Code)
	}
	var out serverDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "online" {
		t.Errorf("status = %q, want online", out.Status)
	}
	if out.IP != "203.0.113.7" {
		t.Errorf("ip = %q", out.IP)
	}
	var engines []map[string]any
	_ = json.Unmarshal(out.Engines, &engines)
	if len(engines) != 2 {
		t.Fatalf("engines = %v", engines)
	}
	if engines[0]["running"] != true {
		t.Errorf("xray should be running: %v", engines[0])
	}
}

func TestServers_Stats(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	srv := createServer(t, env, token, "s1")

	rec := env.do(t, http.MethodGet, "/api/servers/"+itoa(srv.ID)+"/stats", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d", rec.Code)
	}
	var st serverStatsDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.CPUPercent < 20 || st.CPUPercent > 21 {
		t.Errorf("cpu = %v, want ~20.5", st.CPUPercent)
	}
	if st.MemPercent < 34 || st.MemPercent > 35 {
		t.Errorf("mem = %v, want ~34.2", st.MemPercent)
	}
	if st.UptimeSeconds != 864000 {
		t.Errorf("uptime = %d", st.UptimeSeconds)
	}
}

func TestServers_ProvisionInstalls(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	srv := createServer(t, env, token, "s1")

	// run provisioning synchronously via the service
	got, err := env.servers.Provision(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProvisionStatus != "installed" {
		t.Fatalf("provision_status = %q, want installed", got.ProvisionStatus)
	}
	if !env.runner.Ran("openssl req -x509") {
		t.Error("expected self-signed cert generation during provisioning")
	}
}

func TestServers_RequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/servers", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
