package protocols

import (
	"encoding/json"
	"fmt"
	"net/url"
)

func init() { Register(&trojan{}) }

type trojan struct{}

func (trojan) Name() string   { return "trojan" }
func (trojan) Engine() Engine { return EngineXray }

func (trojan) BuildInbound(in Inbound, clients []Client) (json.RawMessage, error) {
	arr := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		arr = append(arr, map[string]any{"password": c.Password, "email": c.Name})
	}
	settings := map[string]any{"clients": arr}
	return json.Marshal(xrayInboundFragment(in, settings))
}

func (trojan) BuildLink(srv Server, in Inbound, c Client) (string, error) {
	q := linkQuery(in.StreamSettings)
	remark := linkRemark(srv, in)
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s",
		url.QueryEscape(c.Password), srv.Host, in.Port, q.Encode(), url.QueryEscape(remark)), nil
}
