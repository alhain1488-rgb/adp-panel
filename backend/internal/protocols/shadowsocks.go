package protocols

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

func init() { Register(&shadowsocks{}) }

type shadowsocks struct{}

func (shadowsocks) Name() string   { return "shadowsocks" }
func (shadowsocks) Engine() Engine { return EngineXray }

func (shadowsocks) BuildInbound(in Inbound, clients []Client) (json.RawMessage, error) {
	method := mstrDefault(in.Settings, "method", "aes-256-gcm")
	network := mstrDefault(in.Settings, "network", "tcp,udp")
	password := ""
	if len(clients) > 0 {
		password = clients[0].Password
	}
	settings := map[string]any{
		"method":   method,
		"password": password,
		"network":  network,
	}
	return json.Marshal(xrayInboundFragment(in, settings))
}

func (shadowsocks) BuildLink(srv Server, in Inbound, c Client) (string, error) {
	method := mstrDefault(in.Settings, "method", "aes-256-gcm")
	userinfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + c.Password))
	remark := linkRemark(srv, in)
	return fmt.Sprintf("ss://%s@%s:%d#%s",
		userinfo, srv.Host, in.Port, url.QueryEscape(remark)), nil
}
