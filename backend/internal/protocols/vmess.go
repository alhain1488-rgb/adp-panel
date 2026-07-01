package protocols

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func init() { Register(&vmess{}) }

type vmess struct{}

func (vmess) Name() string   { return "vmess" }
func (vmess) Engine() Engine { return EngineXray }

func (vmess) BuildInbound(in Inbound, clients []Client) (json.RawMessage, error) {
	cipher := mstrDefault(in.Settings, "security", "auto")
	arr := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		arr = append(arr, map[string]any{
			"id":       c.UUID,
			"alterId":  0,
			"email":    c.Name,
			"security": cipher,
		})
	}
	settings := map[string]any{"clients": arr}
	return json.Marshal(xrayInboundFragment(in, settings))
}

func (vmess) BuildLink(srv Server, in Inbound, c Client) (string, error) {
	cipher := mstrDefault(in.Settings, "security", "auto")
	network := mstrDefault(in.StreamSettings, "network", "tcp")
	security := mstrDefault(in.StreamSettings, "security", "none")

	var host, path string
	if w := msub(in.StreamSettings, "wsSettings"); w != nil {
		path = mstr(w, "path")
		host = mstr(w, "host")
		if host == "" {
			host = mstr(msub(w, "headers"), "Host")
		}
	}

	tls, sni := "", ""
	if security == "tls" {
		tls = "tls"
		sni = mstr(msub(in.StreamSettings, "tlsSettings"), "serverName")
	}

	obj := map[string]any{
		"v":    "2",
		"ps":   linkRemark(srv, in),
		"add":  srv.Host,
		"port": fmt.Sprintf("%d", in.Port),
		"id":   c.UUID,
		"aid":  "0",
		"scy":  cipher,
		"net":  network,
		"type": "none",
		"host": host,
		"path": path,
		"tls":  tls,
		"sni":  sni,
	}

	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(b), nil
}
