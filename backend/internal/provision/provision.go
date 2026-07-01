// Package provision installs the proxy engines on a freshly added node over SSH
// (SPEC §5.1): xray-core and sing-box (Hysteria2), plus a self-signed TLS cert.
// Debian/Ubuntu only. It is idempotent — already-installed engines are skipped.
package provision

import (
	"context"
	"fmt"
	"strings"

	"github.com/adp/panel/internal/ssh"
)

// Default node-side paths for the self-signed Hysteria2 cert.
const (
	DefaultCertPath = "/etc/sing-box/self.crt"
	DefaultKeyPath  = "/etc/sing-box/self.key"
)

// installEnv gives the vendor install scripts a usable environment over a
// PTY-less SSH session: a TERM value (their colored output calls `tput`, which
// otherwise fails with "No value for $TERM and no -T specified" and can abort the
// script) and a noninteractive apt frontend.
const installEnv = `export TERM=xterm DEBIAN_FRONTEND=noninteractive; `

// CmdAptPrep readies apt before the vendor install scripts run: it waits out any
// dpkg lock held by cloud-init/unattended-upgrades on a freshly booted node,
// refreshes the package lists, and pre-installs the dependencies the installers
// need (the Xray script installs `unzip` without an `apt update` first, which
// fails on nodes with stale lists).
const CmdAptPrep = installEnv +
	`for i in $(seq 1 60); do fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 && sleep 2 || break; done; ` +
	`apt-get update -y || true; ` +
	`apt-get install -y --no-install-recommends ca-certificates curl unzip openssl`

// Commands (exported so tests can assert the sequence).
const (
	CmdOSRelease     = `. /etc/os-release 2>/dev/null; echo "${ID:-unknown}"`
	CmdXrayVersion   = `xray version 2>/dev/null | head -n1 || true`
	CmdInstallXray   = installEnv + `bash -c "$(curl -fsSL https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install`
	CmdSingVersion   = `sing-box version 2>/dev/null | head -n1 || true`
	CmdInstallSing   = installEnv + `bash -c "$(curl -fsSL https://sing-box.app/deb-install.sh)"`
	CmdEnsureCertDir = `mkdir -p /etc/sing-box`
)

// EngineInfo describes an installed engine. JSON tags match the API EngineStatus
// shape so the cached engines_json can be passed through unchanged.
type EngineInfo struct {
	Engine      string `json:"engine"`
	Version     string `json:"version,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Running     bool   `json:"running"`
}

// Result is the outcome of provisioning.
type Result struct {
	OS       string
	Engines  []EngineInfo
	CertPath string
	KeyPath  string
}

// Provisioner installs engines using an ssh.Runner.
type Provisioner struct{}

// New builds a Provisioner.
func New() *Provisioner { return &Provisioner{} }

var supportedOS = map[string]bool{"debian": true, "ubuntu": true}

// Provision installs both engines on the node and generates a self-signed cert.
// host is used as the cert CN; hyService is the systemd unit name for sing-box.
func (p *Provisioner) Provision(ctx context.Context, r ssh.Runner, host, hyService string) (*Result, error) {
	osID, err := p.detectOS(ctx, r)
	if err != nil {
		return nil, err
	}
	if !supportedOS[osID] {
		return nil, fmt.Errorf("provision: unsupported OS %q (only Debian/Ubuntu are supported)", osID)
	}

	if err := p.aptPrep(ctx, r); err != nil {
		return nil, err
	}

	xrayVer, err := p.ensureXray(ctx, r)
	if err != nil {
		return nil, err
	}
	singVer, err := p.ensureSingBox(ctx, r)
	if err != nil {
		return nil, err
	}
	certPath, keyPath, err := p.ensureCert(ctx, r, host)
	if err != nil {
		return nil, err
	}

	if hyService == "" {
		hyService = "sing-box"
	}
	return &Result{
		OS:       osID,
		CertPath: certPath,
		KeyPath:  keyPath,
		Engines: []EngineInfo{
			{Engine: "xray", Version: xrayVer, ServiceName: "xray", Running: true},
			{Engine: "hysteria", Version: singVer, ServiceName: hyService, Running: true},
		},
	}, nil
}

func (p *Provisioner) detectOS(ctx context.Context, r ssh.Runner) (string, error) {
	res, err := r.Run(ctx, CmdOSRelease)
	if err != nil {
		return "", fmt.Errorf("provision: detect OS: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(res.Stdout)), nil
}

// aptPrep updates apt and pre-installs the installers' dependencies.
func (p *Provisioner) aptPrep(ctx context.Context, r ssh.Runner) error {
	res, err := r.Run(ctx, CmdAptPrep)
	if err != nil {
		return fmt.Errorf("provision: apt prep: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("provision: apt prep failed (exit %d): %s", res.ExitCode, tailOut(res))
	}
	return nil
}

func (p *Provisioner) ensureXray(ctx context.Context, r ssh.Runner) (string, error) {
	if v := p.version(ctx, r, CmdXrayVersion); v != "" {
		return v, nil
	}
	if res, err := r.Run(ctx, CmdInstallXray); err != nil {
		return "", fmt.Errorf("provision: install xray: %w", err)
	} else if res.ExitCode != 0 {
		return "", fmt.Errorf("provision: install xray failed (exit %d): %s", res.ExitCode, tailOut(res))
	}
	v := p.version(ctx, r, CmdXrayVersion)
	if v == "" {
		return "", fmt.Errorf("provision: xray not found after install")
	}
	return v, nil
}

func (p *Provisioner) ensureSingBox(ctx context.Context, r ssh.Runner) (string, error) {
	if v := p.version(ctx, r, CmdSingVersion); v != "" {
		return v, nil
	}
	if res, err := r.Run(ctx, CmdInstallSing); err != nil {
		return "", fmt.Errorf("provision: install sing-box: %w", err)
	} else if res.ExitCode != 0 {
		return "", fmt.Errorf("provision: install sing-box failed (exit %d): %s", res.ExitCode, tailOut(res))
	}
	v := p.version(ctx, r, CmdSingVersion)
	if v == "" {
		return "", fmt.Errorf("provision: sing-box not found after install")
	}
	return v, nil
}

func (p *Provisioner) ensureCert(ctx context.Context, r ssh.Runner, host string) (string, string, error) {
	if _, err := r.Run(ctx, CmdEnsureCertDir); err != nil {
		return "", "", fmt.Errorf("provision: mkdir cert dir: %w", err)
	}
	cmd := genCertCmd(host, DefaultCertPath, DefaultKeyPath)
	res, err := r.Run(ctx, cmd)
	if err != nil {
		return "", "", fmt.Errorf("provision: generate cert: %w", err)
	}
	if res.ExitCode != 0 {
		return "", "", fmt.Errorf("provision: generate cert failed (exit %d): %s", res.ExitCode, tail(res.Stderr))
	}
	return DefaultCertPath, DefaultKeyPath, nil
}

// genCertCmd produces a self-signed cert only if one is not already present
// (idempotent), using the host as CN/SAN.
func genCertCmd(host, cert, key string) string {
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf(
		`test -f %[1]s && test -f %[2]s || openssl req -x509 -newkey rsa:2048 -nodes `+
			`-keyout %[2]s -out %[1]s -days 3650 -subj "/CN=%[3]s" -addext "subjectAltName=DNS:%[3]s"`,
		cert, key, host)
}

func (p *Provisioner) version(ctx context.Context, r ssh.Runner, cmd string) string {
	res, err := r.Run(ctx, cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[len(s)-300:]
	}
	return s
}

// tailOut combines a command's stderr and stdout (vendor install scripts often
// report the real failure on stdout) and returns the tail for error messages.
func tailOut(res ssh.Result) string {
	parts := make([]string, 0, 2)
	if e := strings.TrimSpace(res.Stderr); e != "" {
		parts = append(parts, e)
	}
	if o := strings.TrimSpace(res.Stdout); o != "" {
		parts = append(parts, o)
	}
	return tail(strings.Join(parts, " | "))
}
