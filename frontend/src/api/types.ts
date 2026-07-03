// Friendly aliases over the auto-generated OpenAPI component schemas so the app
// code doesn't reach into `components['schemas'][...]` everywhere. The source of
// truth is docs/openapi.yaml → schema.ts (regenerate with `npm run gen:api`).
import type { components } from './schema'

type S = components['schemas']

export type Engine = S['Engine']
export type Protocol = S['Protocol']
export type ServerStatus = S['ServerStatus']
export type EngineStatus = S['EngineStatus']

export type Admin = S['Admin']
export type AuthTokens = S['AuthTokens']
export type LoginResponse = S['LoginResponse']
export type TotpSetup = S['TotpSetup']

export type Server = S['Server']
export type ServerInput = S['ServerInput']
export type ServerStats = S['ServerStats']

export type Inbound = S['Inbound']
export type InboundInput = S['InboundInput']

export type Client = S['Client']
export type ClientInput = S['ClientInput']
export type ClientGrant = S['ClientGrant']
export type ClientLink = S['ClientLink']

export type AuditLog = S['AuditLog']
export type AuditLogPage = S['AuditLogPage']

export type DashboardSummary = S['DashboardSummary']
export type DashboardServer = S['DashboardServer']

export type Settings = S['Settings']

export type ApiError = S['Error']

export const PROTOCOL_LABELS: Record<Protocol, string> = {
  vless: 'VLESS',
  vmess: 'VMess',
  trojan: 'Trojan',
  shadowsocks: 'Shadowsocks',
  hysteria2: 'Hysteria2',
  amneziawg: 'AmneziaWG',
}

export const PROTOCOL_ENGINE: Record<Protocol, Engine> = {
  vless: 'xray',
  vmess: 'xray',
  trojan: 'xray',
  shadowsocks: 'xray',
  hysteria2: 'hysteria',
  amneziawg: 'amneziawg',
}

export type AmneziaWGConfig = S['AmneziaWGConfig']
