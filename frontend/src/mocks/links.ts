// Mock equivalent of the backend protocol registry's BuildLink: turn an inbound
// + client into a connection URI. Kept deliberately close to what xray/hysteria
// clients expect so the QR codes and subscription look realistic in Phase 1.
import type { Client, Inbound, Server } from './data'

function enc(v: string): string {
  return encodeURIComponent(v)
}

function b64(s: string): string {
  return btoa(unescape(encodeURIComponent(s)))
}

export function buildLink(server: Server, inbound: Inbound, client: Client): string {
  const host = server.host || server.ip || 'localhost'
  const port = inbound.port
  const remark = `${inbound.remark || inbound.tag} @ ${server.name}`
  const ss = inbound.stream_settings as any
  const st = inbound.settings as any

  switch (inbound.protocol) {
    case 'vless': {
      const params = new URLSearchParams({ encryption: 'none', type: ss?.network ?? 'tcp' })
      if (ss?.security === 'reality') {
        const r = ss.realitySettings ?? {}
        params.set('security', 'reality')
        if (r.flow) params.set('flow', r.flow)
        if (r.serverNames?.[0]) params.set('sni', r.serverNames[0])
        if (r.publicKey) params.set('pbk', r.publicKey)
        if (r.shortIds?.[0]) params.set('sid', r.shortIds[0])
      } else if (ss?.security === 'tls') {
        params.set('security', 'tls')
        if (ss.tlsSettings?.serverName) params.set('sni', ss.tlsSettings.serverName)
        if (ss.wsSettings?.path) params.set('path', ss.wsSettings.path)
        if (ss.wsSettings?.headers?.Host) params.set('host', ss.wsSettings.headers.Host)
      }
      return `vless://${client.uuid}@${host}:${port}?${params.toString()}#${enc(remark)}`
    }
    case 'vmess': {
      const conf = {
        v: '2',
        ps: remark,
        add: host,
        port: String(port),
        id: client.uuid,
        aid: '0',
        scy: 'auto',
        net: ss?.network ?? 'tcp',
        type: 'none',
        host: ss?.wsSettings?.headers?.Host ?? '',
        path: ss?.wsSettings?.path ?? '',
        tls: ss?.security === 'tls' ? 'tls' : '',
        sni: ss?.tlsSettings?.serverName ?? '',
      }
      return `vmess://${b64(JSON.stringify(conf))}`
    }
    case 'trojan': {
      const params = new URLSearchParams({ type: ss?.network ?? 'tcp' })
      if (ss?.security === 'tls') {
        params.set('security', 'tls')
        if (ss.tlsSettings?.serverName) params.set('sni', ss.tlsSettings.serverName)
      }
      return `trojan://${enc(client.password)}@${host}:${port}?${params.toString()}#${enc(remark)}`
    }
    case 'shadowsocks': {
      const method = st?.method ?? 'aes-256-gcm'
      const userinfo = b64(`${method}:${client.password}`)
      return `ss://${userinfo}@${host}:${port}#${enc(remark)}`
    }
    case 'hysteria2': {
      const params = new URLSearchParams()
      const sni = ss?.tlsSettings?.serverName ?? host
      params.set('sni', sni)
      if (ss?.tlsSettings?.insecure) params.set('insecure', '1')
      if (st?.obfs?.type) {
        params.set('obfs', st.obfs.type)
        if (st.obfs.password) params.set('obfs-password', st.obfs.password)
      }
      return `hysteria2://${enc(client.password)}@${host}:${port}/?${params.toString()}#${enc(remark)}`
    }
    default:
      return ''
  }
}
