// The client portal is public (no admin bearer): a subscriber logs in with their
// name + subscription token and gets a read-only view of their subscription.
import type { Protocol } from './types'

export interface PortalLink {
  inbound_id: number
  server_name?: string
  protocol: Protocol
  remark?: string
  uri: string
}

export interface PortalAWG {
  inbound_id: number
  tag: string
  server_name: string
  server_host: string
  conf: string
  vpn_link: string
}

export interface PortalData {
  name: string
  subscription_url: string
  links: PortalLink[]
  amneziawg?: PortalAWG[]
}

export async function portalLogin(name: string, token: string): Promise<PortalData> {
  const res = await fetch('/api/portal/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, token }),
  })
  if (!res.ok) {
    let msg = 'Ошибка входа (Login failed)'
    try {
      const e = (await res.json()) as { error?: string }
      if (e?.error) msg = e.error
    } catch {
      // ignore parse errors, keep the default message
    }
    throw new Error(msg)
  }
  return (await res.json()) as PortalData
}
