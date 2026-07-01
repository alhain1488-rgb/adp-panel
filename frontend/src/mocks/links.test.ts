import { describe, expect, it } from 'vitest'
import { buildLink } from './links'
import { clients, inbounds, servers } from './data'
import type { Client } from './data'

const client: Client = clients[0]

function linkFor(inboundId: number): string {
  const ib = inbounds.find((i) => i.id === inboundId)!
  const srv = servers.find((s) => s.id === ib.server_id)!
  return buildLink(srv, ib, client)
}

describe('buildLink', () => {
  it('builds a vless reality uri with pbk/sid/flow', () => {
    const uri = linkFor(1) // vless-reality-de
    expect(uri.startsWith('vless://')).toBe(true)
    expect(uri).toContain(client.uuid)
    expect(uri).toContain('security=reality')
    expect(uri).toContain('pbk=')
    expect(uri).toContain('flow=xtls-rprx-vision')
  })

  it('builds a vmess base64 uri', () => {
    const uri = linkFor(2) // vmess-ws-de
    expect(uri.startsWith('vmess://')).toBe(true)
    const decoded = JSON.parse(atob(uri.slice('vmess://'.length)))
    expect(decoded.id).toBe(client.uuid)
    expect(decoded.net).toBe('ws')
  })

  it('builds a hysteria2 uri using the client password', () => {
    const uri = linkFor(3) // hy2-de
    expect(uri.startsWith('hysteria2://')).toBe(true)
    expect(uri).toContain(encodeURIComponent(client.password))
    expect(uri).toContain('sni=')
  })

  it('builds a trojan uri', () => {
    const uri = linkFor(5) // trojan-nl
    expect(uri.startsWith('trojan://')).toBe(true)
    expect(uri).toContain('security=tls')
  })

  it('builds a shadowsocks uri with base64 userinfo', () => {
    const uri = linkFor(7) // ss-us
    expect(uri.startsWith('ss://')).toBe(true)
    const at = uri.indexOf('@')
    const userinfo = atob(uri.slice('ss://'.length, at))
    expect(userinfo).toContain('aes-256-gcm:')
  })
})
