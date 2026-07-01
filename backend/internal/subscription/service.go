// Package subscription builds the public, token-addressed subscription: the
// base64-encoded list of connection URIs for all of a client's allowed and
// enabled inbounds across every server (including hysteria2://).
package subscription

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/store"
)

// Service assembles subscriptions from a token.
type Service struct {
	store   *store.Store
	clients *clients.Service
}

// NewService builds a subscription Service.
func NewService(st *store.Store, cs *clients.Service) *Service {
	return &Service{store: st, clients: cs}
}

// Links resolves a token to its client's connection URIs. A disabled client
// yields no links. Returns store.ErrNotFound for an unknown token.
func (s *Service) Links(ctx context.Context, token string) ([]clients.Link, error) {
	c, err := s.store.GetClientByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return []clients.Link{}, nil
	}
	return s.clients.Links(ctx, c.ID)
}

// Build returns the base64-encoded, newline-separated subscription body for a
// token (the format proxy clients expect). An unknown token is store.ErrNotFound.
func (s *Service) Build(ctx context.Context, token string) (string, error) {
	links, err := s.Links(ctx, token)
	if err != nil {
		return "", err
	}
	uris := make([]string, 0, len(links))
	for _, l := range links {
		uris = append(uris, l.URI)
	}
	body := strings.Join(uris, "\n")
	return base64.StdEncoding.EncodeToString([]byte(body)), nil
}
