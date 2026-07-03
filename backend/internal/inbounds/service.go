// Package inbounds manages inbound CRUD. It validates the protocol against the
// registry and generates Reality keys on the backend (never trusting client
// input for secrets).
package inbounds

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/adp/panel/internal/amneziawg"
	"github.com/adp/panel/internal/protocols"
	"github.com/adp/panel/internal/store"
)

// defaultAWGPort is the listen port used for a new AmneziaWG inbound if unset.
const defaultAWGPort = 51820

// ErrUnknownProtocol is returned for a protocol not in the registry.
var ErrUnknownProtocol = errors.New("inbounds: unknown protocol")

// Service manages inbounds.
type Service struct {
	store *store.Store
}

// NewService builds an inbounds Service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Input holds create/update fields with settings as parsed maps.
type Input struct {
	Tag            string
	Protocol       string
	Listen         string
	Port           int
	Settings       map[string]any
	StreamSettings map[string]any
	Sniffing       map[string]any
	Remark         string
	Enabled        bool
}

func marshal(m map[string]any) string {
	if m == nil {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (s *Service) toParams(in Input) store.InboundParams {
	return store.InboundParams{
		Tag:                in.Tag,
		Protocol:           in.Protocol,
		Listen:             in.Listen,
		Port:               in.Port,
		SettingsJSON:       marshal(in.Settings),
		StreamSettingsJSON: marshal(in.StreamSettings),
		SniffingJSON:       marshal(in.Sniffing),
		Remark:             in.Remark,
		Enabled:            in.Enabled,
	}
}

// ensureRealityKeys generates the X25519 keypair + short id for a VLESS Reality
// inbound if they are missing or still placeholders.
func ensureRealityKeys(in *Input) error {
	if in.Protocol != "vless" || in.StreamSettings == nil {
		return nil
	}
	if sec, _ := in.StreamSettings["security"].(string); sec != "reality" {
		return nil
	}
	r, _ := in.StreamSettings["realitySettings"].(map[string]any)
	if r == nil {
		r = map[string]any{}
		in.StreamSettings["realitySettings"] = r
	}
	priv, _ := r["privateKey"].(string)
	if priv == "" || placeholder(priv) {
		keys, err := protocols.GenerateRealityKeys()
		if err != nil {
			return err
		}
		r["privateKey"] = keys.PrivateKey
		r["publicKey"] = keys.PublicKey
	}
	if needShortID(r["shortIds"]) {
		sid, err := protocols.GenerateShortID()
		if err != nil {
			return err
		}
		r["shortIds"] = []any{sid}
	}
	return nil
}

// ensureAmneziaWG fills in the server-side interface secrets and 2.0 defaults
// for an AmneziaWG inbound: a server keypair, tunnel subnet, MTU/DNS/NAT and the
// obfuscation param-set. Client input never sets these; they are generated once
// and reused (the obfuscation set must be identical for every client).
func ensureAmneziaWG(in *Input) error {
	if in.Protocol != "amneziawg" {
		return nil
	}
	if in.Port == 0 {
		in.Port = defaultAWGPort
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	s := in.Settings

	priv, _ := s["private_key"].(string)
	if priv == "" || placeholder(priv) {
		pk, pub, err := amneziawg.GenerateKey()
		if err != nil {
			return err
		}
		s["private_key"], s["public_key"] = pk, pub
	} else if pub, _ := s["public_key"].(string); pub == "" {
		pub, err := amneziawg.PublicFromPrivate(priv)
		if err != nil {
			return err
		}
		s["public_key"] = pub
	}

	setDefault(s, "subnet", "10.9.9.0/24")
	setDefault(s, "dns", "1.1.1.1")
	if _, ok := s["mtu"]; !ok {
		s["mtu"] = 1280
	}
	if _, ok := s["nat"]; !ok {
		s["nat"] = true
	}
	if _, ok := s["params"].(map[string]any); !ok {
		b, _ := json.Marshal(amneziawg.DefaultParams())
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		s["params"] = m
	}
	return nil
}

func setDefault(m map[string]any, key, val string) {
	if v, ok := m[key].(string); !ok || v == "" {
		m[key] = val
	}
}

func placeholder(s string) bool {
	return len(s) > 0 && s[0] == '<'
}

func needShortID(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return true
	}
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" && !placeholder(s) {
			return false
		}
	}
	return true
}

// Create validates and inserts an inbound.
func (s *Service) Create(ctx context.Context, serverID int64, in Input) (*store.Inbound, error) {
	if _, ok := protocols.Get(in.Protocol); !ok {
		return nil, ErrUnknownProtocol
	}
	if err := ensureRealityKeys(&in); err != nil {
		return nil, err
	}
	if err := ensureAmneziaWG(&in); err != nil {
		return nil, err
	}
	return s.store.CreateInbound(ctx, serverID, s.toParams(in))
}

// Update validates and updates an inbound.
func (s *Service) Update(ctx context.Context, id int64, in Input) (*store.Inbound, error) {
	if _, ok := protocols.Get(in.Protocol); !ok {
		return nil, ErrUnknownProtocol
	}
	if err := ensureRealityKeys(&in); err != nil {
		return nil, err
	}
	if err := ensureAmneziaWG(&in); err != nil {
		return nil, err
	}
	return s.store.UpdateInbound(ctx, id, s.toParams(in))
}

// Get / ListByServer / Delete delegate to the store.
func (s *Service) Get(ctx context.Context, id int64) (*store.Inbound, error) {
	return s.store.GetInbound(ctx, id)
}
func (s *Service) ListByServer(ctx context.Context, serverID int64) ([]store.Inbound, error) {
	return s.store.ListInboundsByServer(ctx, serverID)
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.DeleteInbound(ctx, id)
}
