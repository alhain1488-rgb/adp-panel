// Package clients manages subscribers: CRUD, credential generation (uuid,
// password, subscription token), inbound access grants, and connection links.
// Secrets are generated on the backend; client input never sets them.
package clients

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/adp/panel/internal/protocols"
	"github.com/adp/panel/internal/store"
)

// Service manages clients.
type Service struct {
	store *store.Store
}

// NewService builds a clients Service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Input holds the writable client fields.
type Input struct {
	Name   string
	Remark string
}

// Link is a ready-to-use connection URI for one inbound.
type Link struct {
	InboundID  int64
	ServerName string
	Protocol   string
	Remark     string
	URI        string
}

// randToken returns n random bytes as a URL-safe, unpadded base64 string.
func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// randHex returns n random bytes hex-encoded (used for proxy passwords).
func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func genCredentials() (uuidStr, password, token string, err error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", "", "", err
	}
	if password, err = randHex(16); err != nil {
		return "", "", "", err
	}
	if token, err = randToken(24); err != nil {
		return "", "", "", err
	}
	return id.String(), password, token, nil
}

// Create generates credentials and inserts a client (enabled by default).
func (s *Service) Create(ctx context.Context, in Input) (*store.Client, error) {
	id, password, token, err := genCredentials()
	if err != nil {
		return nil, err
	}
	return s.store.CreateClient(ctx, store.ClientParams{
		Name:              in.Name,
		UUID:              id,
		Password:          password,
		SubscriptionToken: token,
		Enabled:           true,
		Remark:            in.Remark,
	})
}

// Get / List / Delete delegate to the store.
func (s *Service) Get(ctx context.Context, id int64) (*store.Client, error) {
	return s.store.GetClient(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]store.Client, error) { return s.store.ListClients(ctx) }
func (s *Service) Delete(ctx context.Context, id int64) error       { return s.store.DeleteClient(ctx, id) }

// Update changes name/remark.
func (s *Service) Update(ctx context.Context, id int64, in Input) (*store.Client, error) {
	if _, err := s.store.GetClient(ctx, id); err != nil {
		return nil, err
	}
	return s.store.UpdateClient(ctx, id, in.Name, in.Remark)
}

// SetEnabled toggles a client on/off.
func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) (*store.Client, error) {
	return s.store.SetClientEnabled(ctx, id, enabled)
}

// SetInbounds replaces the client's grants with the desired set.
func (s *Service) SetInbounds(ctx context.Context, id int64, inboundIDs []int64) (*store.Client, error) {
	if _, err := s.store.GetClient(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.SetClientInbounds(ctx, id, inboundIDs); err != nil {
		return nil, err
	}
	return s.store.GetClient(ctx, id)
}

// GrantedInboundIDs returns the client's granted inbound ids.
func (s *Service) GrantedInboundIDs(ctx context.Context, id int64) ([]int64, error) {
	return s.store.GrantedInboundIDs(ctx, id)
}

// Grants returns the client's grants with server/inbound context.
func (s *Service) Grants(ctx context.Context, id int64) ([]store.ClientGrant, error) {
	return s.store.ListClientGrants(ctx, id)
}

// RotateToken issues a fresh subscription token.
func (s *Service) RotateToken(ctx context.Context, id int64) (*store.Client, error) {
	token, err := randToken(24)
	if err != nil {
		return nil, err
	}
	return s.store.RotateClientToken(ctx, id, token)
}

// Links builds the connection URIs for a client across all its active inbounds.
func (s *Service) Links(ctx context.Context, id int64) ([]Link, error) {
	c, err := s.store.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}
	inbounds, err := s.store.ListActiveClientInbounds(ctx, id)
	if err != nil {
		return nil, err
	}
	return BuildLinks(inbounds, c), nil
}

// BuildLinks turns a client's active inbounds into connection URIs, skipping any
// inbound whose protocol is not registered or fails to build.
func BuildLinks(inbounds []store.InboundWithServer, c *store.Client) []Link {
	pc := protocols.Client{Name: c.Name, UUID: c.UUID, Password: c.Password}
	out := make([]Link, 0, len(inbounds))
	for i := range inbounds {
		iws := inbounds[i]
		adapter, ok := protocols.Get(iws.Protocol)
		if !ok {
			continue
		}
		uri, err := adapter.BuildLink(protocols.Server{Host: iws.ServerHost}, toProtoInbound(iws.Inbound), pc)
		if err != nil {
			continue
		}
		out = append(out, Link{
			InboundID:  iws.ID,
			ServerName: iws.ServerName,
			Protocol:   iws.Protocol,
			Remark:     iws.Remark,
			URI:        uri,
		})
	}
	return out
}

// toProtoInbound converts a stored inbound into the protocol-layer shape by
// parsing its JSON columns.
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
