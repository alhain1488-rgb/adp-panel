package servers

import (
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/ssh/sshtest"
	"github.com/adp/panel/internal/store"
)

func newTestService(t *testing.T, dialer ssh.Dialer) (*Service, *store.Store) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cipher, _ := crypto.NewCipher(key)
	st := store.New(database)
	return NewService(st, cipher, dialer, NoopGeo{}), st
}

func TestCheck_DialFailureMarksError(t *testing.T) {
	dialer := &sshtest.MockDialer{DialErr: errors.New("connection timed out")}
	svc, _ := newTestService(t, dialer)

	srv, err := svc.Create(context.Background(), Input{
		Name: "s1", Host: "10.0.0.1", SSHAuthMethod: "password", SSHSecret: "pw",
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := svc.Check(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "error" {
		t.Errorf("status = %q, want error", out.Status)
	}
}

func TestCreate_EncryptsSecret(t *testing.T) {
	svc, st := newTestService(t, &sshtest.MockDialer{})
	srv, err := svc.Create(context.Background(), Input{
		Name: "s1", Host: "10.0.0.1", SSHAuthMethod: "password", SSHSecret: "supersecret",
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := st.GetServer(context.Background(), srv.ID)
	if stored.SSHSecretEnc == "" || stored.SSHSecretEnc == "supersecret" {
		t.Errorf("ssh secret should be encrypted, got %q", stored.SSHSecretEnc)
	}
}

func TestParseStats(t *testing.T) {
	out := "CPU1 cpu 100 0 50 800 20 0 5 0\n" +
		"CPU2 cpu 110 0 55 860 22 0 6 0\n" +
		"MEM 1400 4096\n" +
		"DISK 17000000 60000000\n" +
		"UPTIME 864000\n"
	st := parseStats(out)
	if st.CPUPercent < 20 || st.CPUPercent > 21 {
		t.Errorf("cpu = %v, want ~20.5", st.CPUPercent)
	}
	if st.MemTotalMB != 4096 || st.MemUsedMB != 1400 {
		t.Errorf("mem = %v/%v", st.MemUsedMB, st.MemTotalMB)
	}
	if st.UptimeSeconds != 864000 {
		t.Errorf("uptime = %d", st.UptimeSeconds)
	}
	if st.DiskTotalGB < 57 || st.DiskTotalGB > 58 {
		t.Errorf("disk total = %v, want ~57.2", st.DiskTotalGB)
	}
}
