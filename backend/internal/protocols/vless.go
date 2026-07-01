package protocols

import (
	"encoding/json"
	"fmt"
	"net/url"
)

func init() { Register(&vless{}) }

type vless struct{}

func (vless) Name() string   { return "vless" }
func (vless) Engine() Engine { return EngineXray }

func (vless) BuildInbound(in Inbound, clients []Client) (json.RawMessage, error) {
	flow := mstr(in.Settings, "flow")
	arr := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		entry := map[string]any{"id": c.UUID, "email": c.Name}
		if flow != "" {
			entry["flow"] = flow
		}
		arr = append(arr, entry)
	}
	settings := map[string]any{"clients": arr, "decryption": "none"}
	return json.Marshal(xrayInboundFragment(in, settings))
}

func (vless) BuildLink(srv Server, in Inbound, c Client) (string, error) {
	q := linkQuery(in.StreamSettings)
	q.Set("encryption", "none")
	if flow := mstr(in.Settings, "flow"); flow != "" {
		q.Set("flow", flow)
	}
	remark := linkRemark(srv, in)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		c.UUID, srv.Host, in.Port, q.Encode(), url.QueryEscape(remark)), nil
}

// linkRemark builds the "#fragment" label for a URI.
func linkRemark(srv Server, in Inbound) string {
	name := in.Remark
	if name == "" {
		name = in.Tag
	}
	if srv.Host != "" {
		return name + " @ " + srv.Host
	}
	return name
}
