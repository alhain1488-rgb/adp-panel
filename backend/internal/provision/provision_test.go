package provision

import (
	"context"
	"strings"
	"testing"

	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/ssh/sshtest"
)

// scriptedRunner builds a mock whose version queries flip to installed once the
// corresponding install command has run.
func scriptedRunner(osID string, xrayPre, singPre bool) *sshtest.MockRunner {
	xray, sing := xrayPre, singPre
	r := &sshtest.MockRunner{}
	r.Handler = func(cmd, _ string) (ssh.Result, error) {
		switch {
		case cmd == CmdOSRelease:
			return ssh.Result{Stdout: osID + "\n"}, nil
		case cmd == CmdXrayVersion:
			if xray {
				return ssh.Result{Stdout: "Xray 1.8.24 (Xray, Penetrates Everything.)\n"}, nil
			}
			return ssh.Result{}, nil
		case cmd == CmdInstallXray:
			xray = true
			return ssh.Result{}, nil
		case cmd == CmdSingVersion:
			if sing {
				return ssh.Result{Stdout: "sing-box version 1.10.0\n"}, nil
			}
			return ssh.Result{}, nil
		case cmd == CmdInstallSing:
			sing = true
			return ssh.Result{}, nil
		default:
			return ssh.Result{}, nil // mkdir, openssl, etc.
		}
	}
	return r
}

func TestProvision_FreshInstall(t *testing.T) {
	r := scriptedRunner("ubuntu", false, false)
	res, err := New().Provision(context.Background(), r, "node.example.com", "sing-box", false)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if r.Ran("[clean] done") {
		t.Error("cleanup must not run when wipe is false")
	}
	if res.OS != "ubuntu" {
		t.Errorf("OS = %q", res.OS)
	}
	if !r.Ran("install-release.sh") {
		t.Error("expected xray installer to run")
	}
	if !r.Ran("deb-install.sh") {
		t.Error("expected sing-box installer to run")
	}
	if !r.Ran("openssl req -x509") {
		t.Error("expected self-signed cert generation")
	}
	if len(res.Engines) != 3 {
		t.Fatalf("engines = %d, want 3 (xray, hysteria, amneziawg)", len(res.Engines))
	}
	if !strings.Contains(res.Engines[0].Version, "1.8.24") {
		t.Errorf("xray version = %q", res.Engines[0].Version)
	}
	if res.Engines[2].Engine != "amneziawg" {
		t.Errorf("third engine = %q, want amneziawg", res.Engines[2].Engine)
	}
	if res.CertPath != DefaultCertPath {
		t.Errorf("cert path = %q", res.CertPath)
	}
}

func TestProvision_UnsupportedOS(t *testing.T) {
	r := scriptedRunner("centos", false, false)
	_, err := New().Provision(context.Background(), r, "node", "sing-box", false)
	if err == nil || !strings.Contains(err.Error(), "unsupported OS") {
		t.Fatalf("expected unsupported OS error, got %v", err)
	}
	if r.Ran("install-release.sh") {
		t.Error("must not install on unsupported OS")
	}
}

func TestProvision_WipeRunsCleanupBeforeInstall(t *testing.T) {
	r := scriptedRunner("ubuntu", false, false)
	if _, err := New().Provision(context.Background(), r, "node", "sing-box", true); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if !r.Ran("[clean] done") {
		t.Error("expected node cleanup to run when wipe is true")
	}
	if !r.Ran("systemctl daemon-reload") {
		t.Error("expected cleanup to remove unit files and reload systemd")
	}
	// Cleanup must not stop the actual install from proceeding.
	if !r.Ran("install-release.sh") || !r.Ran("deb-install.sh") {
		t.Error("engines must still install after cleanup")
	}
}

func TestProvision_WipeSkippedOnUnsupportedOS(t *testing.T) {
	r := scriptedRunner("centos", false, false)
	if _, err := New().Provision(context.Background(), r, "node", "sing-box", true); err == nil {
		t.Fatal("expected unsupported OS error")
	}
	if r.Ran("[clean] done") {
		t.Error("cleanup must not run on an unsupported OS")
	}
}

func TestProvision_Idempotent(t *testing.T) {
	r := scriptedRunner("debian", true, true) // both already installed
	res, err := New().Provision(context.Background(), r, "node", "sing-box", false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Ran("install-release.sh") || r.Ran("deb-install.sh") {
		t.Error("must not reinstall when engines are present")
	}
	if len(res.Engines) != 3 {
		t.Errorf("engines = %d, want 3", len(res.Engines))
	}
}

func TestParseAWGStatus(t *testing.T) {
	cases := []struct {
		out         string
		wantVersion string
		wantRunning bool
	}{
		{"[awg] tools=/usr/bin/awg-quick kmod=yes go=none", "kernel module", true},
		{"[awg] tools=/usr/bin/awg-quick kmod=no go=/usr/local/bin/amneziawg-go", "userspace (amneziawg-go)", true},
		{"[awg] tools=none kmod=no go=none", "not installed", false},
	}
	for _, c := range cases {
		got := ParseAWGStatus(c.out)
		if got.Engine != "amneziawg" {
			t.Errorf("engine = %q", got.Engine)
		}
		if got.Version != c.wantVersion || got.Running != c.wantRunning {
			t.Errorf("ParseAWGStatus(%q) = {%q, running=%v}, want {%q, running=%v}",
				c.out, got.Version, got.Running, c.wantVersion, c.wantRunning)
		}
	}
}
