// Package servers implements server management: CRUD, provisioning (engine
// install), reachability checks, metrics, and engine restart. SSH and geo are
// behind interfaces so the service is unit-tested with mocks.
package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/provision"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/store"
)

// Service orchestrates server operations.
type Service struct {
	store  *store.Store
	cipher *crypto.Cipher
	dialer ssh.Dialer
	prov   *provision.Provisioner
	geo    GeoLocator
}

// NewService builds a servers Service.
func NewService(st *store.Store, cipher *crypto.Cipher, dialer ssh.Dialer, geo GeoLocator) *Service {
	return &Service{store: st, cipher: cipher, dialer: dialer, prov: provision.New(), geo: geo}
}

// Input holds create/update fields (secret is plaintext, write-only).
type Input struct {
	Name                string
	Host                string
	SSHPort             int
	SSHUser             string
	SSHAuthMethod       string
	SSHSecret           string
	SSHPassphrase       string
	XrayConfigPath      string
	XrayServiceName     string
	HysteriaConfigPath  string
	HysteriaServiceName string
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (s *Service) paramsFrom(in Input, secretEnc, passEnc string) store.ServerParams {
	port := in.SSHPort
	if port == 0 {
		port = 22
	}
	return store.ServerParams{
		Name:                in.Name,
		Host:                in.Host,
		SSHPort:             port,
		SSHUser:             firstNonEmpty(in.SSHUser, "root"),
		SSHAuthMethod:       firstNonEmpty(in.SSHAuthMethod, "key"),
		SSHSecretEnc:        secretEnc,
		SSHPassphraseEnc:    passEnc,
		XrayConfigPath:      firstNonEmpty(in.XrayConfigPath, "/usr/local/etc/xray/config.json"),
		XrayServiceName:     firstNonEmpty(in.XrayServiceName, "xray"),
		HysteriaConfigPath:  firstNonEmpty(in.HysteriaConfigPath, "/etc/sing-box/config.json"),
		HysteriaServiceName: firstNonEmpty(in.HysteriaServiceName, "sing-box"),
	}
}

func (s *Service) encrypt(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	return s.cipher.Encrypt(v)
}

// Create inserts a server with its secrets encrypted at rest.
func (s *Service) Create(ctx context.Context, in Input) (*store.Server, error) {
	secretEnc, err := s.encrypt(in.SSHSecret)
	if err != nil {
		return nil, err
	}
	passEnc, err := s.encrypt(in.SSHPassphrase)
	if err != nil {
		return nil, err
	}
	return s.store.CreateServer(ctx, s.paramsFrom(in, secretEnc, passEnc))
}

// Get / List / Delete delegate to the store.
func (s *Service) Get(ctx context.Context, id int64) (*store.Server, error) {
	return s.store.GetServer(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]store.Server, error) { return s.store.ListServers(ctx) }
func (s *Service) Delete(ctx context.Context, id int64) error       { return s.store.DeleteServer(ctx, id) }

// Update changes a server. A blank SSHSecret/Passphrase keeps the existing one.
func (s *Service) Update(ctx context.Context, id int64, in Input) (*store.Server, error) {
	existing, err := s.store.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	secretEnc := existing.SSHSecretEnc
	if in.SSHSecret != "" {
		if secretEnc, err = s.encrypt(in.SSHSecret); err != nil {
			return nil, err
		}
	}
	passEnc := existing.SSHPassphraseEnc
	if in.SSHPassphrase != "" {
		if passEnc, err = s.encrypt(in.SSHPassphrase); err != nil {
			return nil, err
		}
	}
	return s.store.UpdateServer(ctx, id, s.paramsFrom(in, secretEnc, passEnc))
}

// target builds an ssh.Target from a stored server, decrypting secrets.
func (s *Service) target(srv *store.Server) (ssh.Target, error) {
	secret := ""
	if srv.SSHSecretEnc != "" {
		v, err := s.cipher.Decrypt(srv.SSHSecretEnc)
		if err != nil {
			return ssh.Target{}, fmt.Errorf("servers: decrypt ssh secret: %w", err)
		}
		secret = v
	}
	pass := ""
	if srv.SSHPassphraseEnc != "" {
		if v, err := s.cipher.Decrypt(srv.SSHPassphraseEnc); err == nil {
			pass = v
		}
	}
	return ssh.Target{
		Host:       srv.Host,
		Port:       srv.SSHPort,
		User:       srv.SSHUser,
		Auth:       ssh.AuthMethod(srv.SSHAuthMethod),
		Secret:     secret,
		Passphrase: pass,
	}, nil
}

// Provision installs engines on the node (auto after create, or via /install).
func (s *Service) Provision(ctx context.Context, id int64) (*store.Server, error) {
	srv, err := s.store.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.store.SetProvision(ctx, id, "installing", "", "")

	fail := func(e error) (*store.Server, error) {
		_ = s.store.SetProvision(ctx, id, "failed", e.Error(), "")
		return s.store.GetServer(ctx, id)
	}

	target, err := s.target(srv)
	if err != nil {
		return fail(err)
	}
	runner, err := s.dialer.Dial(ctx, target)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = runner.Close() }()

	res, err := s.prov.Provision(ctx, runner, srv.Host, srv.HysteriaServiceName)
	if err != nil {
		return fail(err)
	}
	enginesJSON, _ := json.Marshal(res.Engines)
	_ = s.store.SetProvision(ctx, id, "installed", "", string(enginesJSON))
	return s.store.GetServer(ctx, id)
}

// StartProvision marks the node "installing" and runs provisioning in the
// background (installs take minutes on a real node). Callers return immediately.
func (s *Service) StartProvision(id int64) {
	_ = s.store.SetProvision(context.Background(), id, "installing", "", "")
	go func() { _, _ = s.Provision(context.Background(), id) }()
}

// Check tests reachability and refreshes ip/geo/engine status.
func (s *Service) Check(ctx context.Context, id int64) (*store.Server, error) {
	srv, err := s.store.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	target, err := s.target(srv)
	if err != nil {
		_ = s.store.ApplyCheck(ctx, id, store.CheckUpdate{Status: "error", EnginesJSON: srv.EnginesJSON})
		return s.store.GetServer(ctx, id)
	}
	runner, err := s.dialer.Dial(ctx, target)
	if err != nil {
		_ = s.store.ApplyCheck(ctx, id, store.CheckUpdate{
			Status: "error", IP: srv.IP, GeoCountry: srv.GeoCountry, GeoCity: srv.GeoCity,
			GeoASN: srv.GeoASN, EnginesJSON: srv.EnginesJSON,
		})
		return s.store.GetServer(ctx, id)
	}
	defer func() { _ = runner.Close() }()

	engines := s.detectEngines(ctx, runner, srv)
	enginesJSON, _ := json.Marshal(engines)

	ip := s.nodeIP(ctx, runner)
	geo := Geo{Country: srv.GeoCountry, City: srv.GeoCity, ASN: srv.GeoASN}
	if ip != "" && s.geo != nil {
		if g, err := s.geo.Lookup(ctx, ip); err == nil && g.Country != "" {
			geo = g
		}
	}

	_ = s.store.ApplyCheck(ctx, id, store.CheckUpdate{
		Status: "online", IP: ip, GeoCountry: geo.Country, GeoCity: geo.City, GeoASN: geo.ASN,
		EnginesJSON: string(enginesJSON),
	})
	return s.store.GetServer(ctx, id)
}

// Stats collects host metrics over SSH.
func (s *Service) Stats(ctx context.Context, id int64) (*Stats, error) {
	srv, err := s.store.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	target, err := s.target(srv)
	if err != nil {
		return nil, err
	}
	runner, err := s.dialer.Dial(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("servers: connect: %w", err)
	}
	defer func() { _ = runner.Close() }()

	res, err := runner.Run(ctx, metricsCmd)
	if err != nil {
		return nil, fmt.Errorf("servers: collect metrics: %w", err)
	}
	st := parseStats(res.Stdout)
	return &st, nil
}

// RestartEngine restarts one or all engine services.
func (s *Service) RestartEngine(ctx context.Context, id int64, engine string) error {
	srv, err := s.store.GetServer(ctx, id)
	if err != nil {
		return err
	}
	target, err := s.target(srv)
	if err != nil {
		return err
	}
	runner, err := s.dialer.Dial(ctx, target)
	if err != nil {
		return fmt.Errorf("servers: connect: %w", err)
	}
	defer func() { _ = runner.Close() }()

	var services []string
	switch engine {
	case "xray":
		services = []string{srv.XrayServiceName}
	case "hysteria":
		services = []string{srv.HysteriaServiceName}
	default:
		services = []string{srv.XrayServiceName, srv.HysteriaServiceName}
	}
	for _, svc := range services {
		if _, err := runner.Run(ctx, fmt.Sprintf("systemctl restart %s", svc)); err != nil {
			return fmt.Errorf("servers: restart %s: %w", svc, err)
		}
	}
	return nil
}

func (s *Service) detectEngines(ctx context.Context, r ssh.Runner, srv *store.Server) []provision.EngineInfo {
	return []provision.EngineInfo{
		{
			Engine:      "xray",
			ServiceName: srv.XrayServiceName,
			Running:     s.isActive(ctx, r, srv.XrayServiceName),
			Version:     s.trimRun(ctx, r, "xray version 2>/dev/null | head -n1 || true"),
		},
		{
			Engine:      "hysteria",
			ServiceName: srv.HysteriaServiceName,
			Running:     s.isActive(ctx, r, srv.HysteriaServiceName),
			Version:     s.trimRun(ctx, r, "sing-box version 2>/dev/null | head -n1 || true"),
		},
	}
}

func (s *Service) isActive(ctx context.Context, r ssh.Runner, service string) bool {
	res, err := r.Run(ctx, fmt.Sprintf("systemctl is-active %s 2>/dev/null || true", service))
	return err == nil && strings.TrimSpace(res.Stdout) == "active"
}

func (s *Service) nodeIP(ctx context.Context, r ssh.Runner) string {
	res, err := r.Run(ctx, "curl -s -m 5 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}'")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func (s *Service) trimRun(ctx context.Context, r ssh.Runner, cmd string) string {
	res, err := r.Run(ctx, cmd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}
