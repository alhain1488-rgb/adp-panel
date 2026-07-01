package protocols

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	hyCertPath = "/etc/sing-box/self.crt"
	hyKeyPath  = "/etc/sing-box/self.key"
)

func init() { Register(&hysteria2{}) }

type hysteria2 struct{}

func (hysteria2) Name() string   { return "hysteria2" }
func (hysteria2) Engine() Engine { return EngineHysteria }

// parseMbps extracts the leading integer from a bandwidth value that may be a
// number (float64/int) or a string like "100 mbps". Defaults to 0.
func parseMbps(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		s := strings.TrimSpace(t)
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0
		}
		n, _ := strconv.Atoi(s[:i])
		return n
	default:
		return 0
	}
}

func (hysteria2) BuildInbound(in Inbound, clients []Client) (json.RawMessage, error) {
	users := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		users = append(users, map[string]any{"name": c.Name, "password": c.Password})
	}

	obj := map[string]any{
		"type":        "hysteria2",
		"tag":         in.Tag,
		"listen":      "::",
		"listen_port": in.Port,
		"users":       users,
		"up_mbps":     parseMbps(in.Settings["up"]),
		"down_mbps":   parseMbps(in.Settings["down"]),
		"tls": map[string]any{
			"enabled":          true,
			"alpn":             []string{"h3"},
			"certificate_path": hyCertPath,
			"key_path":         hyKeyPath,
		},
	}

	if obfs := msub(in.Settings, "obfs"); obfs != nil && mstr(obfs, "type") == "salamander" {
		obj["obfs"] = map[string]any{
			"type":     "salamander",
			"password": mstr(obfs, "password"),
		}
	}

	return json.Marshal(obj)
}

func (hysteria2) BuildLink(srv Server, in Inbound, c Client) (string, error) {
	q := url.Values{}

	t := msub(in.StreamSettings, "tlsSettings")
	sni := mstr(t, "serverName")
	if sni == "" {
		sni = srv.Host
	}
	q.Set("sni", sni)
	if mbool(t, "insecure") {
		q.Set("insecure", "1")
	}

	if obfs := msub(in.Settings, "obfs"); obfs != nil && mstr(obfs, "type") == "salamander" {
		q.Set("obfs", "salamander")
		q.Set("obfs-password", mstr(obfs, "password"))
	}

	remark := linkRemark(srv, in)
	return fmt.Sprintf("hysteria2://%s@%s:%d/?%s#%s",
		url.QueryEscape(c.Password), srv.Host, in.Port, q.Encode(), url.QueryEscape(remark)), nil
}
