package protocols

import "net/url"

// buildXrayStream turns the stored stream_settings into the server-side
// streamSettings object xray-core expects (client-only fields like publicKey,
// fingerprint and spiderX are dropped).
func buildXrayStream(ss map[string]any) map[string]any {
	network := mstrDefault(ss, "network", "tcp")
	security := mstrDefault(ss, "security", "none")
	out := map[string]any{"network": network, "security": security}

	switch network {
	case "ws":
		w := msub(ss, "wsSettings")
		headers := map[string]any{}
		if host := mstr(w, "host"); host != "" {
			headers["Host"] = host
		} else if h := msub(w, "headers"); h != nil {
			if host := mstr(h, "Host"); host != "" {
				headers["Host"] = host
			}
		}
		out["wsSettings"] = map[string]any{"path": mstrDefault(w, "path", "/"), "headers": headers}
	case "grpc":
		g := msub(ss, "grpcSettings")
		out["grpcSettings"] = map[string]any{"serviceName": mstr(g, "serviceName"), "multiMode": mbool(g, "multiMode")}
	case "httpupgrade":
		h := msub(ss, "httpupgradeSettings")
		out["httpupgradeSettings"] = map[string]any{"path": mstrDefault(h, "path", "/"), "host": mstr(h, "host")}
	case "xhttp":
		h := msub(ss, "xhttpSettings")
		out["xhttpSettings"] = map[string]any{"path": mstrDefault(h, "path", "/"), "host": mstr(h, "host"), "mode": mstrDefault(h, "mode", "auto")}
	case "tcp":
		if t := msub(ss, "tcpSettings"); t != nil {
			if hdr := msub(t, "header"); mstr(hdr, "type") == "http" {
				out["tcpSettings"] = map[string]any{"header": map[string]any{"type": "http"}}
			}
		}
	}

	switch security {
	case "reality":
		r := msub(ss, "realitySettings")
		out["realitySettings"] = map[string]any{
			"show":        false,
			"dest":        mstrDefault(r, "dest", "www.yahoo.com:443"),
			"xver":        0,
			"serverNames": mstrslice(r, "serverNames"),
			"privateKey":  mstr(r, "privateKey"),
			"shortIds":    mstrslice(r, "shortIds"),
		}
	case "tls":
		t := msub(ss, "tlsSettings")
		tls := map[string]any{"serverName": mstr(t, "serverName")}
		if alpn := mstrslice(t, "alpn"); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		out["tlsSettings"] = tls
	}
	return out
}

// linkQuery builds the shared URI query for xray transport/security. The caller
// adds protocol-specific params (e.g. vless flow).
func linkQuery(ss map[string]any) url.Values {
	q := url.Values{}
	network := mstrDefault(ss, "network", "tcp")
	security := mstrDefault(ss, "security", "none")
	q.Set("type", network)

	switch network {
	case "ws":
		w := msub(ss, "wsSettings")
		if p := mstrDefault(w, "path", "/"); p != "" {
			q.Set("path", p)
		}
		host := mstr(w, "host")
		if host == "" {
			host = mstr(msub(w, "headers"), "Host")
		}
		if host != "" {
			q.Set("host", host)
		}
	case "grpc":
		if sn := mstr(msub(ss, "grpcSettings"), "serviceName"); sn != "" {
			q.Set("serviceName", sn)
		}
	case "httpupgrade", "xhttp":
		sub := msub(ss, network+"Settings")
		if p := mstrDefault(sub, "path", "/"); p != "" {
			q.Set("path", p)
		}
		if h := mstr(sub, "host"); h != "" {
			q.Set("host", h)
		}
	}

	switch security {
	case "reality":
		r := msub(ss, "realitySettings")
		q.Set("security", "reality")
		if sn := mstrslice(r, "serverNames"); len(sn) > 0 {
			q.Set("sni", sn[0])
		}
		if pbk := mstr(r, "publicKey"); pbk != "" {
			q.Set("pbk", pbk)
		}
		if sids := mstrslice(r, "shortIds"); len(sids) > 0 {
			q.Set("sid", sids[0])
		}
		if fp := mstr(r, "fingerprint"); fp != "" {
			q.Set("fp", fp)
		}
	case "tls":
		t := msub(ss, "tlsSettings")
		q.Set("security", "tls")
		if sn := mstr(t, "serverName"); sn != "" {
			q.Set("sni", sn)
		}
		if fp := mstr(t, "fingerprint"); fp != "" {
			q.Set("fp", fp)
		}
	}
	return q
}

// xrayInboundFragment wraps a protocol settings/stream/sniffing into the common
// xray inbound object.
func xrayInboundFragment(in Inbound, settings map[string]any) map[string]any {
	frag := map[string]any{
		"tag":            in.Tag,
		"listen":         orDefault(in.Listen, "0.0.0.0"),
		"port":           in.Port,
		"protocol":       in.Protocol,
		"settings":       settings,
		"streamSettings": buildXrayStream(in.StreamSettings),
	}
	if len(in.Sniffing) > 0 {
		frag["sniffing"] = in.Sniffing
	}
	return frag
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
