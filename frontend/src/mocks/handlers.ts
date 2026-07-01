import { http, HttpResponse } from 'msw'
import QRCode from 'qrcode'
import {
  auditLogs,
  clients,
  dashboardSummary,
  inbounds,
  servers,
  serverStats,
  settings,
  type Client,
  type Inbound,
  type Server,
} from './data'
import { buildLink } from './links'

// ------------------------------------------------------------------ helpers

const json = HttpResponse.json
const notFound = (msg = 'not found') => json({ error: msg }, { status: 404 })

function requireAuth(request: Request) {
  const auth = request.headers.get('Authorization')
  if (!auth || !auth.startsWith('Bearer ')) return false
  return true
}
const unauthorized = () => json({ error: 'authentication required' }, { status: 401 })

function nextId(rows: { id: number }[]): number {
  return rows.reduce((m, r) => Math.max(m, r.id), 0) + 1
}

function randHex(len: number): string {
  const bytes = new Uint8Array(len / 2)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

function recomputeGrants(c: Client) {
  c.grants = c.inbound_ids.map((iid) => {
    const ib = inbounds.find((i) => i.id === iid)
    const srv = ib ? servers.find((s) => s.id === ib.server_id) : undefined
    return {
      inbound_id: iid,
      server_id: ib?.server_id ?? 0,
      server_name: srv?.name ?? 'unknown',
      inbound_tag: ib?.tag ?? 'unknown',
      protocol: ib?.protocol ?? 'vless',
      enabled: ib?.enabled ?? false,
    }
  })
}

function subUrl(token: string): string {
  return `${settings.subscription_base_url}/sub/${token}`
}

// Simulate the provisioning module (SPEC §5.1): mark the node "installing", then
// after a short delay flip to "installed" and expose the freshly installed engines.
function scheduleProvision(srv: Server, delayMs = 3000) {
  srv.provision_status = 'installing'
  srv.provision_error = null
  setTimeout(() => {
    srv.provision_status = 'installed'
    srv.provision_error = null
    if (!srv.engines || srv.engines.length === 0) {
      srv.engines = [
        { engine: 'xray', running: true, version: '1.8.24', service_name: 'xray' },
        { engine: 'hysteria', running: true, version: 'sing-box 1.10.0', service_name: 'sing-box' },
      ]
    }
    srv.updated_at = new Date().toISOString()
  }, delayMs)
}

async function qrPng(text: string): Promise<Uint8Array> {
  const dataUrl = await QRCode.toDataURL(text, { margin: 1, width: 320 })
  const base64 = dataUrl.split(',')[1]
  const bin = atob(base64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return bytes
}

// mutable admin identity
const admin = { id: 1, username: 'admin', totp_enabled: true, created_at: servers[0].created_at, updated_at: new Date().toISOString() }

// ------------------------------------------------------------------ handlers

export const handlers = [
  // ---- Health ----
  http.get('/healthz', () => json({ status: 'ok', version: '0.1.0-mock' })),
  http.get('/readyz', () => json({ status: 'ok', version: '0.1.0-mock' })),

  // ---- Auth ----
  http.post('/api/auth/login', async ({ request }) => {
    const body = (await request.json()) as { username?: string; password?: string }
    if (!body.username || !body.password || body.password === 'wrong') {
      return json({ error: 'invalid username or password' }, { status: 401 })
    }
    // Always require the TOTP step in the mock to exercise the 2FA screen.
    return json({ need_2fa: true, challenge_id: randHex(16) })
  }),
  http.post('/api/auth/2fa/verify', async ({ request }) => {
    const body = (await request.json()) as { code?: string }
    if (!body.code || !/^\d{6}$/.test(body.code)) {
      return json({ error: 'invalid code' }, { status: 401 })
    }
    return json({ token: randHex(48), expires_at: new Date(Date.now() + 3600_000).toISOString() })
  }),
  http.post('/api/auth/2fa/setup', async () => {
    const secret = randHex(32).toUpperCase()
    const otpauth = `otpauth://totp/XrayPanel:${admin.username}?secret=${secret}&issuer=XrayPanel`
    const qr = await QRCode.toDataURL(otpauth, { margin: 1, width: 240 })
    return json({ otpauth_url: otpauth, qr, secret })
  }),
  http.post('/api/auth/2fa/enable', async ({ request }) => {
    const body = (await request.json()) as { code?: string }
    if (!body.code || !/^\d{6}$/.test(body.code)) return json({ error: 'invalid code' }, { status: 401 })
    admin.totp_enabled = true
    return new HttpResponse(null, { status: 204 })
  }),
  http.post('/api/auth/logout', () => new HttpResponse(null, { status: 204 })),
  http.get('/api/auth/me', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    return json(admin)
  }),

  // ---- Dashboard ----
  http.get('/api/dashboard/summary', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    return json(dashboardSummary())
  }),

  // ---- Servers ----
  http.get('/api/servers', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    return json(servers)
  }),
  http.post('/api/servers', async ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    const input = (await request.json()) as any
    const now = new Date().toISOString()
    const srv: Server = {
      id: nextId(servers),
      name: input.name,
      host: input.host,
      ssh_port: input.ssh_port ?? 22,
      ssh_user: input.ssh_user,
      ssh_auth_method: input.ssh_auth_method ?? 'key',
      xray_config_path: input.xray_config_path ?? '/usr/local/etc/xray/config.json',
      xray_service_name: input.xray_service_name ?? 'xray',
      ip: undefined,
      geo_country: undefined,
      geo_city: undefined,
      geo_asn: undefined,
      status: 'unknown',
      provision_status: 'installing',
      provision_error: null,
      engines: [],
      inbound_count: 0,
      last_check_at: null,
      last_sync_at: null,
      last_sync_error: null,
      created_at: now,
      updated_at: now,
    }
    servers.push(srv)
    scheduleProvision(srv)
    return json(srv, { status: 201 })
  }),
  http.get('/api/servers/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    return srv ? json(srv) : notFound()
  }),
  http.put('/api/servers/:id', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    if (!srv) return notFound()
    const input = (await request.json()) as any
    Object.assign(srv, {
      name: input.name ?? srv.name,
      host: input.host ?? srv.host,
      ssh_port: input.ssh_port ?? srv.ssh_port,
      ssh_user: input.ssh_user ?? srv.ssh_user,
      ssh_auth_method: input.ssh_auth_method ?? srv.ssh_auth_method,
      xray_config_path: input.xray_config_path ?? srv.xray_config_path,
      xray_service_name: input.xray_service_name ?? srv.xray_service_name,
      updated_at: new Date().toISOString(),
    })
    return json(srv)
  }),
  http.delete('/api/servers/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const idx = servers.findIndex((s) => s.id === Number(params.id))
    if (idx === -1) return notFound()
    servers.splice(idx, 1)
    return new HttpResponse(null, { status: 204 })
  }),
  http.post('/api/servers/:id/check', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    if (!srv) return notFound()
    const now = new Date().toISOString()
    srv.last_check_at = now
    if (srv.status === 'error') {
      // simulate recovery on manual check
      srv.status = 'online'
      srv.last_sync_error = null
      srv.engines = srv.engines.map((e) => ({ ...e, running: true }))
    }
    if (!srv.ip) {
      srv.ip = `203.0.113.${srv.id * 11}`
      srv.geo_country = srv.geo_country ?? 'Unknown'
    }
    srv.updated_at = now
    return json(srv)
  }),
  http.post('/api/servers/:id/install', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    if (!srv) return notFound()
    scheduleProvision(srv)
    return json(srv)
  }),
  http.post('/api/servers/:id/restart-xray', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    if (!srv) return notFound()
    let engine: string | undefined
    try {
      const body = (await request.json()) as any
      engine = body?.engine
    } catch {
      /* no body */
    }
    srv.engines = srv.engines.map((e) => (!engine || e.engine === engine ? { ...e, running: true } : e))
    srv.updated_at = new Date().toISOString()
    return json({ ok: true, message: `restarted ${engine ?? 'all engines'}`, engine })
  }),
  http.get('/api/servers/:id/stats', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const srv = servers.find((s) => s.id === Number(params.id))
    if (!srv) return notFound()
    return json(serverStats(srv.id))
  }),

  // ---- Inbounds ----
  http.get('/api/servers/:id/inbounds', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const sid = Number(params.id)
    if (!servers.find((s) => s.id === sid)) return notFound()
    return json(inbounds.filter((i) => i.server_id === sid))
  }),
  http.post('/api/servers/:id/inbounds', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const sid = Number(params.id)
    const srv = servers.find((s) => s.id === sid)
    if (!srv) return notFound()
    const input = (await request.json()) as any
    const now = new Date().toISOString()
    const engine = input.protocol === 'hysteria2' ? 'hysteria' : 'xray'
    const ib: Inbound = {
      id: nextId(inbounds),
      server_id: sid,
      tag: input.tag,
      protocol: input.protocol,
      engine,
      listen: input.listen ?? '0.0.0.0',
      port: input.port,
      settings: input.settings ?? {},
      stream_settings: input.stream_settings ?? {},
      sniffing: input.sniffing,
      remark: input.remark,
      enabled: input.enabled ?? true,
      client_count: 0,
      created_at: now,
      updated_at: now,
    }
    inbounds.push(ib)
    srv.inbound_count = inbounds.filter((i) => i.server_id === sid).length
    return json(ib, { status: 201 })
  }),
  http.get('/api/inbounds/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const ib = inbounds.find((i) => i.id === Number(params.id))
    return ib ? json(ib) : notFound()
  }),
  http.put('/api/inbounds/:id', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const ib = inbounds.find((i) => i.id === Number(params.id))
    if (!ib) return notFound()
    const input = (await request.json()) as any
    Object.assign(ib, {
      tag: input.tag ?? ib.tag,
      protocol: input.protocol ?? ib.protocol,
      listen: input.listen ?? ib.listen,
      port: input.port ?? ib.port,
      settings: input.settings ?? ib.settings,
      stream_settings: input.stream_settings ?? ib.stream_settings,
      sniffing: input.sniffing ?? ib.sniffing,
      remark: input.remark ?? ib.remark,
      enabled: input.enabled ?? ib.enabled,
      updated_at: new Date().toISOString(),
    })
    if (input.protocol) ib.engine = input.protocol === 'hysteria2' ? 'hysteria' : 'xray'
    return json(ib)
  }),
  http.delete('/api/inbounds/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const idx = inbounds.findIndex((i) => i.id === Number(params.id))
    if (idx === -1) return notFound()
    const [removed] = inbounds.splice(idx, 1)
    const srv = servers.find((s) => s.id === removed.server_id)
    if (srv) srv.inbound_count = inbounds.filter((i) => i.server_id === srv.id).length
    // drop grants referencing it
    for (const c of clients) {
      if (c.inbound_ids.includes(removed.id)) {
        c.inbound_ids = c.inbound_ids.filter((x) => x !== removed.id)
        recomputeGrants(c)
      }
    }
    return new HttpResponse(null, { status: 204 })
  }),

  // ---- Clients ----
  http.get('/api/clients', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    return json(clients)
  }),
  http.post('/api/clients', async ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    const input = (await request.json()) as any
    const now = new Date().toISOString()
    const token = randHex(64)
    const c: Client = {
      id: nextId(clients),
      name: input.name,
      uuid: crypto.randomUUID(),
      password: randHex(16),
      subscription_token: token,
      subscription_url: subUrl(token),
      enabled: true,
      remark: input.remark,
      inbound_ids: [],
      grants: [],
      created_at: now,
      updated_at: now,
    }
    clients.push(c)
    return json(c, { status: 201 })
  }),
  http.get('/api/clients/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    return c ? json(c) : notFound()
  }),
  http.put('/api/clients/:id', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    const input = (await request.json()) as any
    c.name = input.name ?? c.name
    c.remark = input.remark ?? c.remark
    c.updated_at = new Date().toISOString()
    return json(c)
  }),
  http.delete('/api/clients/:id', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const idx = clients.findIndex((x) => x.id === Number(params.id))
    if (idx === -1) return notFound()
    clients.splice(idx, 1)
    return new HttpResponse(null, { status: 204 })
  }),
  http.post('/api/clients/:id/enable', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    c.enabled = true
    c.updated_at = new Date().toISOString()
    return json(c)
  }),
  http.post('/api/clients/:id/disable', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    c.enabled = false
    c.updated_at = new Date().toISOString()
    return json(c)
  }),
  http.put('/api/clients/:id/inbounds', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    const body = (await request.json()) as { inbound_ids?: number[] }
    const ids = (body.inbound_ids ?? []).filter((iid) => inbounds.some((i) => i.id === iid))
    // update client_count on affected inbounds
    const before = new Set(c.inbound_ids)
    const after = new Set(ids)
    for (const ib of inbounds) {
      const was = before.has(ib.id)
      const now = after.has(ib.id)
      if (was && !now) ib.client_count = Math.max(0, ib.client_count - 1)
      if (!was && now) ib.client_count += 1
    }
    c.inbound_ids = ids
    recomputeGrants(c)
    c.updated_at = new Date().toISOString()
    return json(c)
  }),
  http.post('/api/clients/:id/rotate-token', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    c.subscription_token = randHex(64)
    c.subscription_url = subUrl(c.subscription_token)
    c.updated_at = new Date().toISOString()
    return json(c)
  }),
  http.get('/api/clients/:id/links', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    const links = c.inbound_ids
      .map((iid) => inbounds.find((i) => i.id === iid))
      .filter((ib): ib is Inbound => !!ib && ib.enabled)
      .map((ib) => {
        const srv = servers.find((s) => s.id === ib.server_id)!
        return {
          inbound_id: ib.id,
          server_name: srv.name,
          protocol: ib.protocol,
          remark: ib.remark,
          uri: buildLink(srv, ib, c),
        }
      })
    return json(links)
  }),
  http.get('/api/clients/:id/qrcode', async ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    const url = new URL(request.url)
    const text = url.searchParams.get('link') || c.subscription_url
    const png = await qrPng(text)
    return new HttpResponse(png, { headers: { 'Content-Type': 'image/png' } })
  }),
  http.get('/api/clients/:id/config', ({ request, params }) => {
    if (!requireAuth(request)) return unauthorized()
    const c = clients.find((x) => x.id === Number(params.id))
    if (!c) return notFound()
    const uris = c.inbound_ids
      .map((iid) => inbounds.find((i) => i.id === iid))
      .filter((ib): ib is Inbound => !!ib && ib.enabled)
      .map((ib) => buildLink(servers.find((s) => s.id === ib.server_id)!, ib, c))
    const content = uris.join('\n')
    return new HttpResponse(content, {
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Disposition': `attachment; filename="${c.name}.txt"`,
      },
    })
  }),

  // ---- Subscription (public) ----
  http.get('/sub/:token', ({ params }) => {
    const c = clients.find((x) => x.subscription_token === params.token)
    if (!c || !c.enabled) return new HttpResponse('not found', { status: 404 })
    const uris = c.inbound_ids
      .map((iid) => inbounds.find((i) => i.id === iid))
      .filter((ib): ib is Inbound => !!ib && ib.enabled)
      .map((ib) => buildLink(servers.find((s) => s.id === ib.server_id)!, ib, c))
    const b64 = btoa(unescape(encodeURIComponent(uris.join('\n'))))
    return new HttpResponse(b64, { headers: { 'Content-Type': 'text/plain' } })
  }),

  // ---- Logs ----
  http.get('/api/logs', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    const url = new URL(request.url)
    const page = Math.max(1, Number(url.searchParams.get('page') ?? 1))
    const pageSize = Math.min(200, Math.max(1, Number(url.searchParams.get('page_size') ?? 50)))
    const start = (page - 1) * pageSize
    const items = auditLogs.slice(start, start + pageSize)
    return json({ items, total: auditLogs.length, page, page_size: pageSize })
  }),

  // ---- Settings ----
  http.get('/api/settings', ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    return json(settings)
  }),
  http.put('/api/settings', async ({ request }) => {
    if (!requireAuth(request)) return unauthorized()
    const input = (await request.json()) as any
    Object.assign(settings, input)
    return json(settings)
  }),
]
