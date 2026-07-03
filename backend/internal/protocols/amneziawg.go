package protocols

import (
	"encoding/json"
	"errors"
)

// amneziawg is the AmneziaWG 2.0 adapter. Unlike the xray/sing-box protocols,
// AmneziaWG is WireGuard-based: an inbound is a network interface and clients
// are peers. It is therefore NOT compiled into a JSON config fragment and its
// client link is a per-client .conf/vpn:// built with the client's keypair.
// Those live in internal/amneziawg and the dedicated awg sync/distribution
// paths; this adapter exists so the protocol is registered for the UI and for
// engine routing in sync.
type amneziawg struct{}

// ErrEngineHandled signals that this protocol is assembled by its own engine
// path, not by the generic BuildInbound/BuildLink fragment/URI flow.
var ErrEngineHandled = errors.New("protocols: handled by a dedicated engine path")

func (amneziawg) Name() string   { return "amneziawg" }
func (amneziawg) Engine() Engine { return EngineAmneziaWG }

func (amneziawg) BuildInbound(Inbound, []Client) (json.RawMessage, error) {
	return nil, ErrEngineHandled
}

func (amneziawg) BuildLink(Server, Inbound, Client) (string, error) {
	return "", ErrEngineHandled
}

func init() { Register(amneziawg{}) }
