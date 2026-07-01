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
	res, err := New().Provision(context.Background(), r, "node.example.com", "sing-box")
	if err != nil {
		t.Fatalf("provision: %v", err)
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
	if len(res.Engines) != 2 {
		t.Fatalf("engines = %d, want 2", len(res.Engines))
	}
	if !strings.Contains(res.Engines[0].Version, "1.8.24") {
		t.Errorf("xray version = %q", res.Engines[0].Version)
	}
	if res.CertPath != DefaultCertPath {
		t.Errorf("cert path = %q", res.CertPath)
	}
}

func TestProvision_UnsupportedOS(t *testing.T) {
	r := scriptedRunner("centos", false, false)
	_, err := New().Provision(context.Background(), r, "node", "sing-box")
	if err == nil || !strings.Contains(err.Error(), "unsupported OS") {
		t.Fatalf("expected unsupported OS error, got %v", err)
	}
	if r.Ran("install-release.sh") {
		t.Error("must not install on unsupported OS")
	}
}

func TestProvision_Idempotent(t *testing.T) {
	r := scriptedRunner("debian", true, true) // both already installed
	res, err := New().Provision(context.Background(), r, "node", "sing-box")
	if err != nil {
		t.Fatal(err)
	}
	if r.Ran("install-release.sh") || r.Ran("deb-install.sh") {
		t.Error("must not reinstall when engines are present")
	}
	if len(res.Engines) != 2 {
		t.Errorf("engines = %d, want 2", len(res.Engines))
	}
}
