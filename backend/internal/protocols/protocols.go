// Package protocols is the single place that knows how to turn an inbound into
// engine config and a client connection URI. Adding a protocol = implement
// Protocol and register it; no DB schema change (SPEC §5). Each adapter declares
// its target engine (xray-core or the Hysteria2 engine, sing-box).
package protocols

import "encoding/json"

// Engine is the config target an inbound belongs to.
type Engine string

const (
	EngineXray      Engine = "xray"      // config.json for xray-core
	EngineHysteria  Engine = "hysteria"  // sing-box config for Hysteria2
	EngineAmneziaWG Engine = "amneziawg" // awg-quick .conf for AmneziaWG 2.0
)

// Inbound is the protocol-agnostic inbound definition passed to adapters.
// Settings/StreamSettings/Sniffing are the parsed JSON columns.
type Inbound struct {
	Tag            string
	Protocol       string
	Listen         string
	Port           int
	Remark         string
	Settings       map[string]any
	StreamSettings map[string]any
	Sniffing       map[string]any
}

// Client is a single client's credentials (already filtered to enabled grants).
type Client struct {
	Name     string
	UUID     string
	Password string
}

// Server carries what BuildLink needs about the host.
type Server struct {
	Host string
}

// Protocol is implemented by every supported protocol adapter.
type Protocol interface {
	// Name is the protocol id stored in inbounds.protocol (e.g. "vless").
	Name() string
	// Engine is the config target this inbound is compiled into.
	Engine() Engine
	// BuildInbound assembles the engine config fragment for this inbound and the
	// clients granted to it.
	BuildInbound(in Inbound, clients []Client) (json.RawMessage, error)
	// BuildLink builds a subscription URI for one client on this inbound.
	BuildLink(srv Server, in Inbound, c Client) (string, error)
}

var registry = map[string]Protocol{}

// Register adds an adapter to the registry (called from adapter init()).
func Register(p Protocol) { registry[p.Name()] = p }

// Get returns the adapter for a protocol name.
func Get(name string) (Protocol, bool) {
	p, ok := registry[name]
	return p, ok
}

// EngineOf returns the target engine for a protocol name.
func EngineOf(name string) (Engine, bool) {
	if p, ok := registry[name]; ok {
		return p.Engine(), true
	}
	return "", false
}

// Names returns all registered protocol names.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	return out
}
