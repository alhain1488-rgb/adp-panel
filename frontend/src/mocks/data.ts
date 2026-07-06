// Mock data for the Xray proxy management panel.
// Self-contained: local interfaces mirror the OpenAPI schemas.
// MSW handlers read and mutate the exported arrays.

// ---------------------------------------------------------------- Types

export type Protocol = 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'hysteria2';
export type Engine = 'xray' | 'hysteria';
export type ServerStatus = 'online' | 'offline' | 'unknown' | 'error';
export type ProvisionStatus = 'pending' | 'installing' | 'installed' | 'failed';
export type SshAuthMethod = 'password' | 'key';
export type Theme = 'light' | 'dark' | 'system';
export type HysteriaEngine = 'sing-box' | 'hysteria';

export interface EngineStatus {
  engine: Engine;
  running: boolean;
  version?: string;
  service_name?: string;
}

export interface Server {
  id: number;
  name: string;
  host: string;
  ssh_port: number;
  ssh_user: string;
  ssh_auth_method: SshAuthMethod;
  xray_config_path?: string;
  xray_service_name?: string;
  hysteria_config_path?: string;
  hysteria_service_name?: string;
  ip?: string;
  geo_country?: string;
  geo_city?: string;
  geo_asn?: string;
  status: ServerStatus;
  provision_status?: ProvisionStatus;
  provision_error?: string | null;
  engines: EngineStatus[];
  inbound_count: number;
  last_check_at: string | null;
  last_sync_at: string | null;
  last_sync_error: string | null;
  created_at: string;
  updated_at: string;
}

export interface Inbound {
  id: number;
  server_id: number;
  tag: string;
  protocol: Protocol;
  engine: Engine;
  listen: string;
  port: number;
  settings: Record<string, unknown>;
  stream_settings: Record<string, unknown>;
  sniffing?: Record<string, unknown>;
  remark?: string;
  enabled: boolean;
  client_count: number;
  created_at: string;
  updated_at: string;
}

export interface ClientGrant {
  inbound_id: number;
  server_id: number;
  server_name: string;
  inbound_tag: string;
  protocol: Protocol;
  enabled: boolean;
}

export interface Client {
  id: number;
  name: string;
  uuid: string;
  password: string;
  subscription_token: string;
  subscription_url: string;
  enabled: boolean;
  remark?: string;
  email?: string;
  telegram_linked?: boolean;
  telegram_username?: string;
  inbound_ids: number[];
  grants: ClientGrant[];
  created_at: string;
  updated_at: string;
}

export interface AuditLog {
  id: number;
  admin_id: number;
  admin_username: string;
  action: string;
  target_type?: string;
  target_id?: number;
  detail?: Record<string, unknown>;
  ip?: string;
  user_agent?: string;
  created_at: string;
}

export interface ServerStats {
  cpu_percent: number;
  mem_percent: number;
  mem_used_mb: number;
  mem_total_mb: number;
  disk_percent: number;
  disk_used_gb: number;
  disk_total_gb: number;
  uptime_seconds: number;
  collected_at: string;
}

export interface Settings {
  domain: string;
  subscription_base_url: string;
  sync_interval_seconds: number;
  hysteria_engine: HysteriaEngine;
  theme: Theme;
}

export interface DashboardServer {
  id: number;
  name: string;
  host: string;
  status: ServerStatus;
  geo_country?: string;
  stats: ServerStats;
  engines: EngineStatus[];
  last_sync_at: string | null;
}

export interface DashboardSummary {
  client_count: number;
  enabled_client_count: number;
  server_count: number;
  online_server_count: number;
  inbound_count: number;
  last_sync_at: string | null;
  servers: DashboardServer[];
}

// ---------------------------------------------------------------- Helpers

/** ISO timestamp for "n minutes ago". */
const minsAgo = (n: number): string => new Date(Date.now() - n * 60000).toISOString();

// ---------------------------------------------------------------- Servers

export const servers: Server[] = [
  {
    id: 1,
    name: 'de-frankfurt-1',
    host: 'fra1.example.com',
    ssh_port: 22,
    ssh_user: 'root',
    ssh_auth_method: 'key',
    xray_config_path: '/usr/local/etc/xray/config.json',
    xray_service_name: 'xray',
    ip: '188.245.112.37',
    geo_country: 'Germany',
    geo_city: 'Frankfurt',
    geo_asn: 'AS24940 Hetzner Online GmbH',
    status: 'online',
    engines: [
      { engine: 'xray', running: true, version: '1.8.24', service_name: 'xray' },
      { engine: 'hysteria', running: true, version: 'sing-box 1.10.0', service_name: 'sing-box' },
    ],
    inbound_count: 3,
    last_check_at: minsAgo(2),
    last_sync_at: minsAgo(6),
    last_sync_error: null,
    created_at: minsAgo(60 * 24 * 40),
    updated_at: minsAgo(6),
  },
  {
    id: 2,
    name: 'nl-amsterdam-1',
    host: 'ams1.example.com',
    ssh_port: 22,
    ssh_user: 'root',
    ssh_auth_method: 'key',
    xray_config_path: '/usr/local/etc/xray/config.json',
    xray_service_name: 'xray',
    ip: '45.77.203.18',
    geo_country: 'Netherlands',
    geo_city: 'Amsterdam',
    geo_asn: 'AS20473 The Constant Company (Vultr)',
    status: 'online',
    engines: [
      { engine: 'xray', running: true, version: '1.8.24', service_name: 'xray' },
      { engine: 'hysteria', running: true, version: 'sing-box 1.10.0', service_name: 'sing-box' },
    ],
    inbound_count: 3,
    last_check_at: minsAgo(4),
    last_sync_at: minsAgo(9),
    last_sync_error: null,
    created_at: minsAgo(60 * 24 * 33),
    updated_at: minsAgo(9),
  },
  {
    id: 3,
    name: 'us-newyork-1',
    host: 'nyc1.example.com',
    ssh_port: 22,
    ssh_user: 'root',
    ssh_auth_method: 'password',
    xray_config_path: '/usr/local/etc/xray/config.json',
    xray_service_name: 'xray',
    ip: '104.207.129.62',
    geo_country: 'USA',
    geo_city: 'New York',
    geo_asn: 'AS20473 The Constant Company (Vultr)',
    status: 'online',
    engines: [
      { engine: 'xray', running: true, version: '1.8.24', service_name: 'xray' },
      { engine: 'hysteria', running: false, version: 'sing-box 1.10.0', service_name: 'sing-box' },
    ],
    inbound_count: 2,
    last_check_at: minsAgo(7),
    last_sync_at: minsAgo(15),
    last_sync_error: null,
    created_at: minsAgo(60 * 24 * 21),
    updated_at: minsAgo(15),
  },
  {
    id: 4,
    name: 'jp-tokyo-1',
    host: 'tyo1.example.com',
    ssh_port: 22,
    ssh_user: 'root',
    ssh_auth_method: 'password',
    xray_config_path: '/usr/local/etc/xray/config.json',
    xray_service_name: 'xray',
    ip: '172.104.98.211',
    geo_country: 'Japan',
    geo_city: 'Tokyo',
    geo_asn: 'AS63949 Akamai Connected Cloud (Linode)',
    status: 'error',
    engines: [
      { engine: 'xray', running: false, version: '1.8.24', service_name: 'xray' },
      { engine: 'hysteria', running: false, version: 'sing-box 1.10.0', service_name: 'sing-box' },
    ],
    inbound_count: 1,
    last_check_at: minsAgo(11),
    last_sync_at: minsAgo(180),
    last_sync_error:
      'ssh: handshake failed: dial tcp 172.104.98.211:22: connect: connection timed out',
    created_at: minsAgo(60 * 24 * 12),
    updated_at: minsAgo(11),
  },
];

// Existing seeded nodes are already provisioned (engines installed).
servers.forEach((s) => {
  s.provision_status = 'installed';
  s.provision_error = null;
  s.hysteria_config_path = s.hysteria_config_path ?? '/etc/sing-box/config.json';
  s.hysteria_service_name = s.hysteria_service_name ?? 'sing-box';
});

// ---------------------------------------------------------------- Inbounds

export const inbounds: Inbound[] = [
  // --- Server 1 (Frankfurt) ---
  {
    id: 1,
    server_id: 1,
    tag: 'vless-reality-de',
    protocol: 'vless',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 443,
    settings: {
      clients: [],
      decryption: 'none',
    },
    stream_settings: {
      network: 'tcp',
      security: 'reality',
      realitySettings: {
        show: false,
        dest: 'www.yahoo.com:443',
        xver: 0,
        serverNames: ['www.yahoo.com', 'www.wikipedia.org'],
        privateKey: 'yBb0m5s8Qk3fZ2rN7wJvC1xD9uH4eLpTaG6oM8kXyE',
        publicKey: 'wYf2Kp9Lx7Rq4Zm1Nc8Vd3Bh6Jt0Ss5Aw2Ee9Uu4Oi',
        shortIds: ['6ba85179e30d4fc2', '0a1b2c3d'],
        flow: 'xtls-rprx-vision',
      },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'] },
    remark: 'Reality vision — primary',
    enabled: true,
    client_count: 3,
    created_at: minsAgo(60 * 24 * 40),
    updated_at: minsAgo(120),
  },
  {
    id: 2,
    server_id: 1,
    tag: 'vmess-ws-de',
    protocol: 'vmess',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 8443,
    settings: { clients: [] },
    stream_settings: {
      network: 'ws',
      security: 'tls',
      wsSettings: { path: '/vm', headers: { Host: 'fra1.example.com' } },
      tlsSettings: { serverName: 'fra1.example.com', alpn: ['h2', 'http/1.1'] },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls'] },
    remark: 'VMess over WebSocket + TLS',
    enabled: true,
    client_count: 2,
    created_at: minsAgo(60 * 24 * 39),
    updated_at: minsAgo(300),
  },
  {
    id: 3,
    server_id: 1,
    tag: 'hy2-de',
    protocol: 'hysteria2',
    engine: 'hysteria',
    listen: '0.0.0.0',
    port: 36712,
    settings: {
      obfs: { type: 'salamander', password: 'kL9mZx2Qw7Rt' },
      up: '200 mbps',
      down: '500 mbps',
    },
    stream_settings: {
      security: 'tls',
      tlsSettings: { serverName: 'fra1.example.com', insecure: true },
    },
    remark: 'Hysteria2 — high throughput',
    enabled: true,
    client_count: 2,
    created_at: minsAgo(60 * 24 * 20),
    updated_at: minsAgo(45),
  },

  // --- Server 2 (Amsterdam) ---
  {
    id: 4,
    server_id: 2,
    tag: 'vless-tls-ws-nl',
    protocol: 'vless',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 443,
    settings: { clients: [], decryption: 'none' },
    stream_settings: {
      network: 'ws',
      security: 'tls',
      wsSettings: { path: '/vl', headers: { Host: 'ams1.example.com' } },
      tlsSettings: { serverName: 'ams1.example.com', alpn: ['h2', 'http/1.1'] },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls'] },
    remark: 'VLESS + TLS over WebSocket',
    enabled: true,
    client_count: 2,
    created_at: minsAgo(60 * 24 * 33),
    updated_at: minsAgo(240),
  },
  {
    id: 5,
    server_id: 2,
    tag: 'trojan-nl',
    protocol: 'trojan',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 8443,
    settings: { clients: [] },
    stream_settings: {
      network: 'tcp',
      security: 'tls',
      tlsSettings: { serverName: 'ams1.example.com', alpn: ['h2', 'http/1.1'] },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls'] },
    remark: 'Trojan over TLS',
    enabled: true,
    client_count: 1,
    created_at: minsAgo(60 * 24 * 30),
    updated_at: minsAgo(600),
  },
  {
    id: 6,
    server_id: 2,
    tag: 'hy2-nl',
    protocol: 'hysteria2',
    engine: 'hysteria',
    listen: '0.0.0.0',
    port: 36712,
    settings: {
      obfs: { type: 'salamander', password: 'pQ4vN8Rj1Wz6' },
      up: '150 mbps',
      down: '400 mbps',
    },
    stream_settings: {
      security: 'tls',
      tlsSettings: { serverName: 'ams1.example.com', insecure: true },
    },
    remark: 'Hysteria2 — Amsterdam',
    enabled: true,
    client_count: 1,
    created_at: minsAgo(60 * 24 * 18),
    updated_at: minsAgo(90),
  },

  // --- Server 3 (New York) ---
  {
    id: 7,
    server_id: 3,
    tag: 'ss-us',
    protocol: 'shadowsocks',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 8388,
    settings: {
      method: 'aes-256-gcm',
      password: 'Tz7Kq2Wm9Xr4Vp1Ln6Hs3',
      network: 'tcp,udp',
    },
    stream_settings: { network: 'tcp' },
    sniffing: { enabled: true, destOverride: ['http', 'tls'] },
    remark: 'Shadowsocks aes-256-gcm',
    enabled: true,
    client_count: 2,
    created_at: minsAgo(60 * 24 * 21),
    updated_at: minsAgo(720),
  },
  {
    id: 8,
    server_id: 3,
    tag: 'vless-reality-us',
    protocol: 'vless',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 443,
    settings: { clients: [], decryption: 'none' },
    stream_settings: {
      network: 'tcp',
      security: 'reality',
      realitySettings: {
        show: false,
        dest: 'www.apple.com:443',
        xver: 0,
        serverNames: ['www.apple.com'],
        privateKey: 'aC3dE5fG7hJ9kL1mN3pQ5rS7tU9wX1yZ3bD5fH7jK9',
        publicKey: 'zX9wV7uT5sR3qP1oN9mL7kJ5hG3fE1dC9bA7yZ5xW3',
        shortIds: ['9f8e7d6c5b4a3210'],
        flow: 'xtls-rprx-vision',
      },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'] },
    remark: 'Reality — New York (disabled)',
    enabled: false,
    client_count: 1,
    created_at: minsAgo(60 * 24 * 15),
    updated_at: minsAgo(1440),
  },

  // --- Server 4 (Tokyo, error state) ---
  {
    id: 9,
    server_id: 4,
    tag: 'vless-reality-jp',
    protocol: 'vless',
    engine: 'xray',
    listen: '0.0.0.0',
    port: 443,
    settings: { clients: [], decryption: 'none' },
    stream_settings: {
      network: 'tcp',
      security: 'reality',
      realitySettings: {
        show: false,
        dest: 'www.cloudflare.com:443',
        xver: 0,
        serverNames: ['www.cloudflare.com'],
        privateKey: 'mN3pQ5rS7tU9wX1yZ3bD5fH7jK9aC3dE5fG7hJ9kL1',
        publicKey: 'kJ5hG3fE1dC9bA7yZ5xW3zX9wV7uT5sR3qP1oN9mL7',
        shortIds: ['1a2b3c4d5e6f7080'],
        flow: 'xtls-rprx-vision',
      },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'] },
    remark: 'Reality — Tokyo',
    enabled: true,
    client_count: 1,
    created_at: minsAgo(60 * 24 * 12),
    updated_at: minsAgo(180),
  },
];

// ---------------------------------------------------------------- Clients

/** Build grants from a list of inbound ids using the current inbounds/servers. */
function grantsFor(inboundIds: number[]): ClientGrant[] {
  return inboundIds.map((iid) => {
    const ib = inbounds.find((i) => i.id === iid);
    const srv = ib ? servers.find((s) => s.id === ib.server_id) : undefined;
    return {
      inbound_id: iid,
      server_id: ib ? ib.server_id : 0,
      server_name: srv ? srv.name : 'unknown',
      inbound_tag: ib ? ib.tag : 'unknown',
      protocol: ib ? ib.protocol : 'vless',
      enabled: ib ? ib.enabled : false,
    };
  });
}

export const clients: Client[] = [
  {
    id: 1,
    name: 'nick-laptop',
    uuid: 'b3f1a9c2-4d5e-4f7a-8b1c-2e3d4f5a6b7c',
    password: 'Xk92Lm4Qp7Zr',
    subscription_token: 'a1b2c3d4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00',
    subscription_url:
      'https://panel.example.com/sub/a1b2c3d4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00',
    enabled: true,
    remark: 'Personal laptop — all regions',
    inbound_ids: [1, 3, 4, 7],
    grants: grantsFor([1, 3, 4, 7]),
    created_at: minsAgo(60 * 24 * 38),
    updated_at: minsAgo(120),
  },
  {
    id: 2,
    name: 'nick-phone',
    uuid: 'c4a2b8d3-5e6f-4a8b-9c2d-3f4e5a6b7c8d',
    password: 'Wm38Rt5Yq2Nx',
    subscription_token: 'ff00eeddccbbaa998877665544332211009f8e7d6c5b4a39281706f5e4d3c2b1',
    subscription_url:
      'https://panel.example.com/sub/ff00eeddccbbaa998877665544332211009f8e7d6c5b4a39281706f5e4d3c2b1',
    enabled: true,
    remark: 'iPhone — Hysteria2 preferred',
    telegram_linked: true,
    telegram_username: 'nick_tg',
    inbound_ids: [3, 6],
    grants: grantsFor([3, 6]),
    created_at: minsAgo(60 * 24 * 30),
    updated_at: minsAgo(90),
  },
  {
    id: 3,
    name: 'family-shared',
    uuid: 'd5b3c9e4-6f70-4b9c-ad3e-4a5b6c7d8e9f',
    password: 'Qp71Zx4Vm8Kd',
    subscription_token: '112233445566778899aabbccddeeff00a1b2c3d4e5f60718293a4b5c6d7e8f90',
    subscription_url:
      'https://panel.example.com/sub/112233445566778899aabbccddeeff00a1b2c3d4e5f60718293a4b5c6d7e8f90',
    enabled: true,
    remark: 'Shared with family',
    inbound_ids: [4, 5, 8],
    grants: grantsFor([4, 5, 8]),
    created_at: minsAgo(60 * 24 * 25),
    updated_at: minsAgo(1440),
  },
  {
    id: 4,
    name: 'travel-router',
    uuid: 'e6c4d0f5-7081-4cad-be4f-5b6c7d8e9f01',
    password: 'Nx84Kd2Wm5Qp',
    subscription_token: '7d6c5b4a39281706f5e4d3c2b1a0ff00eeddccbbaa998877665544332211bead',
    subscription_url:
      'https://panel.example.com/sub/7d6c5b4a39281706f5e4d3c2b1a0ff00eeddccbbaa998877665544332211bead',
    enabled: false,
    remark: 'GL.iNet travel router — disabled while stored',
    inbound_ids: [1, 9],
    grants: grantsFor([1, 9]),
    created_at: minsAgo(60 * 24 * 14),
    updated_at: minsAgo(60 * 24 * 3),
  },
  {
    id: 5,
    name: 'spare-unassigned',
    uuid: 'f7d5e1a6-8192-4dbe-cf50-6c7d8e9f0112',
    password: 'Kd27Qp9Nx4Wm',
    subscription_token: '00ffeeddccbbaa99887766554433221100abcdef0123456789fedcba98765432',
    subscription_url:
      'https://panel.example.com/sub/00ffeeddccbbaa99887766554433221100abcdef0123456789fedcba98765432',
    enabled: true,
    remark: 'Spare credential, no inbounds granted yet',
    inbound_ids: [],
    grants: grantsFor([]),
    created_at: minsAgo(60 * 24 * 2),
    updated_at: minsAgo(60 * 24 * 2),
  },
];

// ---------------------------------------------------------------- Audit logs

const UA_DESKTOP =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36';
const UA_MOBILE =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1';

export const auditLogs: AuditLog[] = [
  { id: 40, admin_id: 1, admin_username: 'admin', action: 'login', target_type: 'admin', target_id: 1, detail: { method: 'password+totp' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(3) },
  { id: 39, admin_id: 1, admin_username: 'admin', action: 'server.check', target_type: 'server', target_id: 1, detail: { status: 'online', engines: ['xray', 'hysteria'] }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(4) },
  { id: 38, admin_id: 1, admin_username: 'admin', action: 'server.check', target_type: 'server', target_id: 4, detail: { status: 'error', error: 'connection timed out' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(11) },
  { id: 37, admin_id: 1, admin_username: 'admin', action: 'sync.push', target_type: 'server', target_id: 2, detail: { inbounds: 3, result: 'ok' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(9) },
  { id: 36, admin_id: 1, admin_username: 'admin', action: 'sync.push', target_type: 'server', target_id: 1, detail: { inbounds: 3, result: 'ok' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(6) },
  { id: 35, admin_id: 1, admin_username: 'admin', action: 'client.rotate_token', target_type: 'client', target_id: 1, detail: { client: 'nick-laptop' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(120) },
  { id: 34, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 1, detail: { field: 'remark' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(120) },
  { id: 33, admin_id: 1, admin_username: 'admin', action: 'client.grant_inbounds', target_type: 'client', target_id: 2, detail: { inbound_ids: [3, 6] }, ip: '198.51.100.77', user_agent: UA_MOBILE, created_at: minsAgo(90) },
  { id: 32, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 6, detail: { field: 'settings.down', value: '400 mbps' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(90) },
  { id: 31, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 3, detail: { field: 'enabled', value: true }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(45) },
  { id: 30, admin_id: 1, admin_username: 'admin', action: 'login', target_type: 'admin', target_id: 1, detail: { method: 'password+totp' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(240) },
  { id: 29, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 4, detail: { field: 'stream_settings.wsSettings.path' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(240) },
  { id: 28, admin_id: 1, admin_username: 'admin', action: 'settings.update', target_type: 'settings', detail: { sync_interval_seconds: 300 }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(300) },
  { id: 27, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 2, detail: { field: 'stream_settings' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(300) },
  { id: 26, admin_id: 1, admin_username: 'admin', action: 'server.restart_xray', target_type: 'server', target_id: 3, detail: { engine: 'xray', result: 'ok' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(600) },
  { id: 25, admin_id: 1, admin_username: 'admin', action: 'inbound.update', target_type: 'inbound', target_id: 8, detail: { field: 'enabled', value: false }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(1440) },
  { id: 24, admin_id: 1, admin_username: 'admin', action: 'client.update', target_type: 'client', target_id: 3, detail: { field: 'remark' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(1440) },
  { id: 23, admin_id: 1, admin_username: 'admin', action: 'client.disable', target_type: 'client', target_id: 4, detail: { client: 'travel-router' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 3) },
  { id: 22, admin_id: 1, admin_username: 'admin', action: 'client.create', target_type: 'client', target_id: 5, detail: { name: 'spare-unassigned' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 2) },
  { id: 21, admin_id: 1, admin_username: 'admin', action: 'login', target_type: 'admin', target_id: 1, detail: { method: 'password+totp' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 2) },
  { id: 20, admin_id: 1, admin_username: 'admin', action: 'inbound.create', target_type: 'inbound', target_id: 9, detail: { tag: 'vless-reality-jp', server_id: 4 }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 12) },
  { id: 19, admin_id: 1, admin_username: 'admin', action: 'server.create', target_type: 'server', target_id: 4, detail: { name: 'jp-tokyo-1', host: 'tyo1.example.com' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 12) },
  { id: 18, admin_id: 1, admin_username: 'admin', action: 'client.create', target_type: 'client', target_id: 4, detail: { name: 'travel-router' }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 14) },
  { id: 17, admin_id: 1, admin_username: 'admin', action: 'inbound.create', target_type: 'inbound', target_id: 8, detail: { tag: 'vless-reality-us', server_id: 3 }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 15) },
  { id: 16, admin_id: 1, admin_username: 'admin', action: 'inbound.create', target_type: 'inbound', target_id: 6, detail: { tag: 'hy2-nl', server_id: 2 }, ip: '203.0.113.24', user_agent: UA_DESKTOP, created_at: minsAgo(60 * 24 * 18) },
];

// ---------------------------------------------------------------- Settings

export const settings: Settings = {
  domain: 'panel.example.com',
  subscription_base_url: 'https://panel.example.com',
  sync_interval_seconds: 300,
  hysteria_engine: 'sing-box',
  theme: 'system',
};

// ---------------------------------------------------------------- Stats

/** Deterministic-ish per-server stats so cards differ but stay stable-ish. */
export function serverStats(serverId: number): ServerStats {
  const seed = serverId;
  const cpu = Math.round((12 + seed * 9.3 + (Date.now() / 60000) % 7) * 10) / 10;
  const memTotal = 2048 * seed + 2048; // 4GB, 6GB, 8GB, 10GB
  const memPercent = Math.round((28 + seed * 8.1) * 10) / 10;
  const memUsed = Math.round(memTotal * (memPercent / 100));
  const diskTotal = 40 + seed * 20; // 60, 80, 100, 120 GB
  const diskPercent = Math.round((22 + seed * 6.4) * 10) / 10;
  const diskUsed = Math.round(diskTotal * (diskPercent / 100) * 10) / 10;
  const uptime = 60 * 60 * 24 * (7 * seed + 3) + seed * 3600; // days-ish, varies

  return {
    cpu_percent: Math.min(cpu, 98),
    mem_percent: memPercent,
    mem_used_mb: memUsed,
    mem_total_mb: memTotal,
    disk_percent: diskPercent,
    disk_used_gb: diskUsed,
    disk_total_gb: diskTotal,
    uptime_seconds: uptime,
    collected_at: new Date().toISOString(),
  };
}

// ---------------------------------------------------------------- Dashboard

export function dashboardSummary(): DashboardSummary {
  const onlineCount = servers.filter((s) => s.status === 'online').length;
  const enabledClients = clients.filter((c) => c.enabled).length;
  const inboundTotal = inbounds.length;

  const syncTimes = servers
    .map((s) => s.last_sync_at)
    .filter((t): t is string => t !== null)
    .sort();
  const lastSync = syncTimes.length ? syncTimes[syncTimes.length - 1] : null;

  const dashServers: DashboardServer[] = servers.map((s) => ({
    id: s.id,
    name: s.name,
    host: s.host,
    status: s.status,
    geo_country: s.geo_country,
    stats: serverStats(s.id),
    engines: s.engines,
    last_sync_at: s.last_sync_at,
  }));

  return {
    client_count: clients.length,
    enabled_client_count: enabledClients,
    server_count: servers.length,
    online_server_count: onlineCount,
    inbound_count: inboundTotal,
    last_sync_at: lastSync,
    servers: dashServers,
  };
}
