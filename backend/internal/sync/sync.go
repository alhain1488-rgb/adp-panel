// Package sync assembles each node engine's config from its enabled inbounds and
// their granted clients, then pushes it over SSH: backup, write, validate
// (xray -test / sing-box check), and restart the service. It is multi-engine
// (xray-core and the Hysteria2 engine can both run on one node) and idempotent
// by config hash.
package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/adp/panel/internal/amneziawg"
	"github.com/adp/panel/internal/protocols"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/store"
)

// awgConfigDir is where amneziawg-tools' awg-quick reads interface configs.
const awgConfigDir = "/etc/amnezia/amneziawg"

// ErrNotProvisioned is returned when a node has not finished engine install; the
// sync engine refuses to push to it.
var ErrNotProvisioned = errors.New("sync: server not provisioned")

// Connector opens an SSH runner to a server (implemented by servers.Service).
type Connector interface {
	Connect(ctx context.Context, serverID int64) (ssh.Runner, *store.Server, error)
}

// ClientKeyer returns (generating on first use) a client's WireGuard keypair,
// needed to build AmneziaWG peers. Implemented by clients.Service.
type ClientKeyer interface {
	WGKeypair(ctx context.Context, clientID int64) (priv, pub string, err error)
}

// Service runs the sync engine.
type Service struct {
	store *store.Store
	conn  Connector
	keyer ClientKeyer
}

// NewService builds a sync Service. keyer may be nil when AmneziaWG is not used.
func NewService(st *store.Store, conn Connector, keyer ClientKeyer) *Service {
	return &Service{store: st, conn: conn, keyer: keyer}
}

// EngineResult reports what happened for one engine during a sync.
type EngineResult struct {
	Engine   protocols.Engine
	Changed  bool // config differed and was pushed + restarted
	Skipped  bool // config unchanged (idempotent no-op)
	InboundN int
	ClientN  int
}

// Sync assembles and pushes configs for every engine that has inbounds on the
// server. The outcome is recorded in servers.last_sync_*.
func (s *Service) Sync(ctx context.Context, serverID int64) ([]EngineResult, error) {
	srv, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if srv.ProvisionStatus != "installed" {
		_ = s.store.SetSync(ctx, serverID, ErrNotProvisioned.Error())
		return nil, ErrNotProvisioned
	}

	plans, err := s.plan(ctx, srv)
	if err != nil {
		_ = s.store.SetSync(ctx, serverID, err.Error())
		return nil, err
	}

	runner, _, err := s.conn.Connect(ctx, serverID)
	if err != nil {
		_ = s.store.SetSync(ctx, serverID, err.Error())
		return nil, err
	}
	defer func() { _ = runner.Close() }()

	results := make([]EngineResult, 0, len(plans))
	for _, p := range plans {
		res, err := s.applyEngine(ctx, runner, serverID, p)
		if err != nil {
			msg := fmt.Sprintf("%s: %v", p.engine, err)
			_ = s.store.SetSync(ctx, serverID, msg)
			return results, fmt.Errorf("sync: %s", msg)
		}
		results = append(results, res)
	}

	_ = s.store.SetSync(ctx, serverID, "")
	return results, nil
}

// Async pushes a server's config in the background (fire-and-forget). Errors are
// recorded in the server's last_sync_error. Used to auto-apply config changes.
func (s *Service) Async(serverID int64) {
	go func() { _, _ = s.Sync(context.Background(), serverID) }()
}

// AsyncAll re-syncs every installed server in the background. Sync is idempotent
// by config hash, so unaffected nodes are a cheap no-op. Used after client-level
// changes that may touch inbounds on several servers.
func (s *Service) AsyncAll() {
	go func() {
		ctx := context.Background()
		ids, err := s.store.ListInstalledServerIDs(ctx)
		if err != nil {
			return
		}
		for _, id := range ids {
			_, _ = s.Sync(ctx, id)
		}
	}()
}

// enginePlan is one engine's assembled, ready-to-push config.
type enginePlan struct {
	engine protocols.Engine
	// key identifies this plan for idempotency ("xray"/"hysteria", or per
	// AmneziaWG interface "amneziawg:awgN" since each is a separate service).
	key      string
	path     string
	service  string
	testCmd  func(path string) string
	content  []byte
	inbounds int
	clients  int
}

// plan gathers the server's enabled inbounds, groups them by engine, and builds
// each engine's config. Engines with no inbounds are omitted.
func (s *Service) plan(ctx context.Context, srv *store.Server) ([]enginePlan, error) {
	inbounds, err := s.store.ListEnabledInboundsByServer(ctx, srv.ID)
	if err != nil {
		return nil, err
	}

	xrayFrags := []json.RawMessage{}
	hyFrags := []json.RawMessage{}
	var xrayInb, hyInb, xrayCli, hyCli int
	var plans []enginePlan

	for i := range inbounds {
		in := inbounds[i]
		adapter, ok := protocols.Get(in.Protocol)
		if !ok {
			continue
		}
		grantees, err := s.store.ListInboundGrantedClients(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		// AmneziaWG is WireGuard-based: one interface (its own awg-quick service)
		// per inbound, built directly rather than as a JSON config fragment.
		if adapter.Engine() == protocols.EngineAmneziaWG {
			p, err := s.buildAWGPlan(ctx, in, grantees)
			if err != nil {
				return nil, fmt.Errorf("build amneziawg inbound %q: %w", in.Tag, err)
			}
			plans = append(plans, p)
			continue
		}
		frag, err := adapter.BuildInbound(toProtoInbound(in), toProtoClients(grantees))
		if err != nil {
			return nil, fmt.Errorf("build inbound %q: %w", in.Tag, err)
		}
		switch adapter.Engine() {
		case protocols.EngineXray:
			xrayFrags = append(xrayFrags, frag)
			xrayInb++
			xrayCli += len(grantees)
		case protocols.EngineHysteria:
			hyFrags = append(hyFrags, frag)
			hyInb++
			hyCli += len(grantees)
		}
	}

	if len(xrayFrags) > 0 {
		content, err := buildXrayConfig(xrayFrags)
		if err != nil {
			return nil, err
		}
		plans = append(plans, enginePlan{
			engine: protocols.EngineXray, key: string(protocols.EngineXray),
			path: srv.XrayConfigPath, service: srv.XrayServiceName,
			testCmd: xrayTestCmd, content: content, inbounds: xrayInb, clients: xrayCli,
		})
	}
	if len(hyFrags) > 0 {
		content, err := buildSingboxConfig(hyFrags)
		if err != nil {
			return nil, err
		}
		plans = append(plans, enginePlan{
			engine: protocols.EngineHysteria, key: string(protocols.EngineHysteria),
			path: srv.HysteriaConfigPath, service: srv.HysteriaServiceName,
			testCmd: singboxTestCmd, content: content, inbounds: hyInb, clients: hyCli,
		})
	}
	return plans, nil
}

// buildAWGPlan assembles one AmneziaWG interface's server .conf: the stored
// interface params plus one [Peer] per granted client (each client's WireGuard
// public key and its tunnel IP, derived deterministically from the client id).
func (s *Service) buildAWGPlan(ctx context.Context, in store.Inbound, grantees []store.Client) (enginePlan, error) {
	iface, err := amneziawg.ParseInterface(in.SettingsJSON, in.Port)
	if err != nil {
		return enginePlan{}, err
	}
	if len(grantees) > 0 && s.keyer == nil {
		return enginePlan{}, errors.New("amneziawg: no client keyer configured")
	}
	peers := make([]amneziawg.Peer, 0, len(grantees))
	for _, c := range grantees {
		_, pub, err := s.keyer.WGKeypair(ctx, c.ID)
		if err != nil {
			return enginePlan{}, err
		}
		ip, err := amneziawg.HostIP(iface.Subnet, int(c.ID)+1)
		if err != nil {
			return enginePlan{}, err
		}
		peers = append(peers, amneziawg.Peer{Name: c.Name, PublicKey: pub, Address: ip + "/32"})
	}
	content, err := amneziawg.ServerConfig(iface, peers)
	if err != nil {
		return enginePlan{}, err
	}
	ifaceName := fmt.Sprintf("awg%d", in.ID)
	return enginePlan{
		engine:   protocols.EngineAmneziaWG,
		key:      "amneziawg:" + ifaceName,
		path:     awgConfigDir + "/" + ifaceName + ".conf",
		service:  "awg-quick@" + ifaceName,
		testCmd:  awgTestCmd,
		content:  []byte(content),
		inbounds: 1,
		clients:  len(peers),
	}, nil
}

// applyEngine pushes one engine's config: idempotency check, backup, write,
// validate (restoring the backup on failure), and restart.
func (s *Service) applyEngine(ctx context.Context, r ssh.Runner, serverID int64, p enginePlan) (EngineResult, error) {
	res := EngineResult{Engine: p.engine, InboundN: p.inbounds, ClientN: p.clients}

	hash := sha256Hex(p.content)
	planKey := p.key
	if planKey == "" {
		planKey = string(p.engine)
	}
	key := fmt.Sprintf("sync:%d:%s", serverID, planKey)
	if prev, _ := s.store.GetSetting(ctx, key); prev == hash {
		res.Skipped = true
		return res, nil
	}

	dir := dirOf(p.path)
	backup := p.path + ".adp.bak"

	// Ensure the directory exists, back up any current config, and write the new one.
	writeCmd := fmt.Sprintf("mkdir -p %s && ( cp %s %s 2>/dev/null || true ) && cat > %s",
		shellQuote(dir), shellQuote(p.path), shellQuote(backup), shellQuote(p.path))
	if out, err := r.RunInput(ctx, writeCmd, string(p.content)); err != nil {
		return res, fmt.Errorf("write config: %w", err)
	} else if out.ExitCode != 0 {
		return res, fmt.Errorf("write config: exit %d: %s", out.ExitCode, strings.TrimSpace(out.Stderr))
	}

	// Validate; on failure restore the backup and abort without restarting.
	if out, err := r.Run(ctx, p.testCmd(p.path)); err != nil {
		return res, fmt.Errorf("validate: %w", err)
	} else if out.ExitCode != 0 {
		_, _ = r.Run(ctx, fmt.Sprintf("cp %s %s 2>/dev/null || true", shellQuote(backup), shellQuote(p.path)))
		detail := strings.TrimSpace(out.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(out.Stdout)
		}
		return res, fmt.Errorf("config rejected by engine: %s", detail)
	}

	if out, err := r.Run(ctx, fmt.Sprintf("systemctl restart %s", shellQuote(p.service))); err != nil {
		return res, fmt.Errorf("restart %s: %w", p.service, err)
	} else if out.ExitCode != 0 {
		return res, fmt.Errorf("restart %s: exit %d: %s", p.service, out.ExitCode, strings.TrimSpace(out.Stderr))
	}

	if err := s.store.SetSetting(ctx, key, hash); err != nil {
		return res, err
	}
	res.Changed = true
	return res, nil
}

func xrayTestCmd(path string) string    { return fmt.Sprintf("xray -test -config %s", shellQuote(path)) }
func singboxTestCmd(path string) string { return fmt.Sprintf("sing-box check -c %s", shellQuote(path)) }

// awgTestCmd validates an AmneziaWG config by having awg-quick parse it (strip
// prints the wg-usable form; a syntax error exits non-zero).
func awgTestCmd(path string) string { return fmt.Sprintf("awg-quick strip %s", shellQuote(path)) }

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i > 0 {
		return path[:i]
	}
	return "."
}

// shellQuote single-quotes a string for safe use in a POSIX shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func toProtoInbound(in store.Inbound) protocols.Inbound {
	return protocols.Inbound{
		Tag:            in.Tag,
		Protocol:       in.Protocol,
		Listen:         in.Listen,
		Port:           in.Port,
		Remark:         in.Remark,
		Settings:       parseJSON(in.SettingsJSON),
		StreamSettings: parseJSON(in.StreamSettingsJSON),
		Sniffing:       parseJSON(in.SniffingJSON),
	}
}

func toProtoClients(cs []store.Client) []protocols.Client {
	out := make([]protocols.Client, 0, len(cs))
	for _, c := range cs {
		out = append(out, protocols.Client{Name: c.Name, UUID: c.UUID, Password: c.Password})
	}
	return out
}

func parseJSON(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]any{}
	}
	return m
}

// buildXrayConfig wraps xray inbound fragments into a full config.json.
func buildXrayConfig(frags []json.RawMessage) ([]byte, error) {
	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  frags,
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// buildSingboxConfig wraps hysteria2 inbound fragments into a sing-box config.
func buildSingboxConfig(frags []json.RawMessage) ([]byte, error) {
	cfg := map[string]any{
		"log":       map[string]any{"level": "warn"},
		"inbounds":  frags,
		"outbounds": []any{map[string]any{"type": "direct"}},
	}
	return json.MarshalIndent(cfg, "", "  ")
}
