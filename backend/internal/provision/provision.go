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

// CmdInstallAWG installs the AmneziaWG 2.0 engine on the node (best-effort;
// always exits 0 so a node that won't host AmneziaWG isn't blocked). Primary
// path: the Amnezia PPA's DKMS kernel module (`amneziawg`) + `amneziawg-tools`
// (awg/awg-quick + the awg-quick@ systemd template) — the canonical, Go-free
// install. Fallback (Debian, or a node without matching kernel headers): source
// -build the tools and run the userspace `amneziawg-go`, installing a modern Go
// first since distro Go is often too old for `go install ...@latest`. It also
// enables IP forwarding and creates the config dir. The final `[awg]` line
// reports what actually landed (tools / kmod / go) so a deploy can verify it.
// NOTE: still worth confirming the `[awg]` status output on the target node.
const CmdInstallAWG = installEnv + `
set +e
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1
grep -q '^net.ipv4.ip_forward=1' /etc/sysctl.conf || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
mkdir -p /etc/amnezia/amneziawg
# Primary: Amnezia PPA — DKMS kernel module + tools (no userspace Go needed).
if ! command -v awg-quick >/dev/null 2>&1 || ! modinfo amneziawg >/dev/null 2>&1; then
  apt-get install -y --no-install-recommends software-properties-common python3-launchpadlib gnupg2 iptables dkms build-essential "linux-headers-$(uname -r)" >/dev/null 2>&1
  add-apt-repository -y ppa:amnezia/ppa >/dev/null 2>&1 && apt-get update -y >/dev/null 2>&1
  apt-get install -y amneziawg amneziawg-tools >/dev/null 2>&1
fi
# Fallback: no kernel module (Debian / missing headers / container) -> userspace amneziawg-go.
if ! modinfo amneziawg >/dev/null 2>&1; then
  if ! command -v awg-quick >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends git build-essential iptables >/dev/null 2>&1
    git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools /tmp/awg-tools >/dev/null 2>&1
    make -C /tmp/awg-tools/src >/dev/null 2>&1 && make -C /tmp/awg-tools/src install WITH_SYSTEMDUNITS=yes >/dev/null 2>&1
  fi
  if ! command -v amneziawg-go >/dev/null 2>&1; then
    GO="$(command -v go || true)"
    if [ -z "$GO" ] || ! "$GO" version 2>/dev/null | grep -qE 'go1\.(2[0-9]|[3-9][0-9])'; then
      curl -fsSL "https://go.dev/dl/go1.22.5.linux-$(dpkg --print-architecture).tar.gz" -o /tmp/go.tgz >/dev/null 2>&1 \
        && rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz >/dev/null 2>&1 && GO=/usr/local/go/bin/go
    fi
    [ -n "$GO" ] && HOME=/root GOBIN=/usr/local/bin "$GO" install github.com/amnezia-vpn/amneziawg-go@latest >/dev/null 2>&1
  fi
  mkdir -p /etc/systemd/system/awg-quick@.service.d
  printf '[Service]\nEnvironment=WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go\n' > /etc/systemd/system/awg-quick@.service.d/userspace.conf
fi
systemctl daemon-reload >/dev/null 2>&1
echo "[awg] tools=$(command -v awg-quick || echo none) kmod=$(modinfo amneziawg >/dev/null 2>&1 && echo yes || echo no) go=$(command -v amneziawg-go || echo none)"
exit 0`

// CmdAWGStatus reports the AmneziaWG engine state on a node WITHOUT installing
// anything (used by Check to keep the reported engines consistent with what
// provisioning installed). Same "[awg] tools=.. kmod=.. go=.." shape as the tail
// of CmdInstallAWG, so ParseAWGStatus reads either.
const CmdAWGStatus = `echo "[awg] tools=$(command -v awg-quick || echo none) kmod=$(modinfo amneziawg >/dev/null 2>&1 && echo yes || echo no) go=$(command -v amneziawg-go || echo none)"`

// CmdCleanNode is the optional, opt-in pre-provision cleanup. It removes
// competing proxy/VPN stacks and panels so their services stop squatting the
// ports and memory the panel's engines need. It is deliberately *targeted*: it
// never touches the OS, SSH, networking, or generic web servers (nginx/apache),
// and it is best-effort — it always exits 0, so a stray leftover can never abort
// the install. It stops/disables known units, kills leftover processes, removes
// all Docker containers (frees ports/RAM; Docker itself is kept), and deletes
// known proxy binaries, configs and unit files.
const CmdCleanNode = installEnv + `
set +e
echo "[clean] stopping proxy/VPN/panel services"
for svc in xray v2ray sing-box hysteria hysteria-server hysteria2 trojan trojan-go \
           shadowsocks-libev ss-server shadowsocks-rust snell tuic naiveproxy \
           brook gost xrayr v2raya x-ui 3x-ui s-ui marzban marzban-node \
           wg-quick@wg0 wireguard openvpn openvpn@server; do
  systemctl stop "$svc" >/dev/null 2>&1
  systemctl disable "$svc" >/dev/null 2>&1
done
echo "[clean] killing leftover proxy processes"
for p in xray v2ray sing-box hysteria trojan ss-server ss-local shadowsocks tuic naive brook gost xrayr v2raya; do
  pkill -9 -x "$p" >/dev/null 2>&1
done
echo "[clean] removing docker containers"
if command -v docker >/dev/null 2>&1; then
  ids=$(docker ps -aq 2>/dev/null)
  [ -n "$ids" ] && docker rm -f $ids >/dev/null 2>&1
fi
echo "[clean] removing proxy binaries, configs and unit files"
rm -rf /usr/local/etc/xray /usr/local/bin/xray /usr/local/share/xray \
       /etc/systemd/system/xray.service /etc/systemd/system/xray@.service \
       /usr/local/etc/v2ray /usr/local/bin/v2ray /etc/v2ray \
       /etc/systemd/system/v2ray.service /etc/systemd/system/v2ray@.service \
       /etc/sing-box /usr/bin/sing-box /usr/local/bin/sing-box /etc/systemd/system/sing-box.service \
       /etc/hysteria /usr/local/bin/hysteria /etc/systemd/system/hysteria-server.service \
       /etc/trojan /etc/trojan-go /usr/bin/trojan /usr/local/bin/trojan-go \
       /etc/shadowsocks-libev /usr/bin/ss-server /usr/local/bin/ss-server \
       /usr/local/bin/tuic /usr/local/bin/brook /usr/local/bin/gost \
       /opt/3x-ui /opt/x-ui /etc/x-ui /usr/local/x-ui /opt/s-ui \
       /opt/marzban /opt/marzban-node /opt/marzban-scripts \
       /usr/local/bin/v2raya /etc/systemd/system/v2raya.service /usr/local/bin/xrayr /etc/XrayR \
       >/dev/null 2>&1
systemctl daemon-reload >/dev/null 2>&1
echo "[clean] done"
exit 0`

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
func (p *Provisioner) Provision(ctx context.Context, r ssh.Runner, host, hyService string, wipe bool) (*Result, error) {
	osID, err := p.detectOS(ctx, r)
	if err != nil {
		return nil, err
	}
	if !supportedOS[osID] {
		return nil, fmt.Errorf("provision: unsupported OS %q (only Debian/Ubuntu are supported)", osID)
	}

	// Opt-in: wipe competing proxy stacks before installing our engines.
	if wipe {
		if err := p.clean(ctx, r); err != nil {
			return nil, err
		}
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
	// AmneziaWG is a best-effort third engine: install it, but never fail the
	// whole provision over it (a node that won't host AmneziaWG still installs).
	awg := p.ensureAmneziaWG(ctx, r)

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
			awg,
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

// clean runs the targeted node cleanup (best-effort). The command always exits
// 0, so only a transport error is surfaced — a leftover we couldn't remove must
// not block the install.
func (p *Provisioner) clean(ctx context.Context, r ssh.Runner) error {
	if _, err := r.Run(ctx, CmdCleanNode); err != nil {
		return fmt.Errorf("provision: clean node: %w", err)
	}
	return nil
}

// ensureAmneziaWG installs the AmneziaWG 2.0 engine (best-effort; errors are
// swallowed so provisioning of the core engines is never blocked by it).
func (p *Provisioner) ensureAmneziaWG(ctx context.Context, r ssh.Runner) EngineInfo {
	res, _ := r.Run(ctx, CmdInstallAWG)
	return ParseAWGStatus(res.Stdout)
}

// ParseAWGStatus turns an "[awg] tools=.. kmod=.. go=.." status line into an
// EngineInfo so provisioning and Check report whether AmneziaWG actually landed
// (kernel module vs userspace vs none) instead of silently discarding the result.
// Running == the awg-quick tool is present; Version reflects the implementation.
func ParseAWGStatus(out string) EngineInfo {
	tools, kmod, gobin := "", "", ""
	for _, f := range strings.Fields(out) {
		switch {
		case strings.HasPrefix(f, "tools="):
			tools = strings.TrimPrefix(f, "tools=")
		case strings.HasPrefix(f, "kmod="):
			kmod = strings.TrimPrefix(f, "kmod=")
		case strings.HasPrefix(f, "go="):
			gobin = strings.TrimPrefix(f, "go=")
		}
	}
	toolsOK := tools != "" && tools != "none"
	ver := "not installed"
	switch {
	case kmod == "yes":
		ver = "kernel module"
	case gobin != "" && gobin != "none":
		ver = "userspace (amneziawg-go)"
	}
	return EngineInfo{
		Engine:      "amneziawg",
		Version:     ver,
		ServiceName: "awg-quick@",
		Running:     toolsOK && ver != "not installed",
	}
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
