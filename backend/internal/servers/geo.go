package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Geo is coarse IP geolocation.
type Geo struct {
	Country string
	City    string
	ASN     string
}

// GeoLocator resolves an IP to a location. Implementations are best-effort.
type GeoLocator interface {
	Lookup(ctx context.Context, ip string) (Geo, error)
}

// NoopGeo returns empty geo (used when lookups are disabled or in tests).
type NoopGeo struct{}

// Lookup implements GeoLocator.
func (NoopGeo) Lookup(context.Context, string) (Geo, error) { return Geo{}, nil }

// IPAPIGeo looks up geo via the free ip-api.com service.
type IPAPIGeo struct {
	Client *http.Client
}

// NewIPAPIGeo builds an IPAPIGeo with a bounded HTTP client.
func NewIPAPIGeo() *IPAPIGeo {
	return &IPAPIGeo{Client: &http.Client{Timeout: 5 * time.Second}}
}

// Lookup implements GeoLocator.
func (g *IPAPIGeo) Lookup(ctx context.Context, ip string) (Geo, error) {
	if ip == "" {
		return Geo{}, nil
	}
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,city,as", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Geo{}, err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return Geo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Status  string `json:"status"`
		Country string `json:"country"`
		City    string `json:"city"`
		AS      string `json:"as"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Geo{}, err
	}
	if body.Status != "success" {
		return Geo{}, nil
	}
	return Geo{Country: body.Country, City: body.City, ASN: body.AS}, nil
}
