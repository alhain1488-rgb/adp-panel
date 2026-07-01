# Absolutely Disgusting Panel

A self-hosted web panel to manage several VPS running **Xray** and **Hysteria2**
proxies from one place — for **personal use**. One panel on a main VPS controls the
proxies on all your nodes over SSH: servers, inbounds of many protocols, clients,
and their subscriptions. No billing, traffic limits, or multi-tenancy — by design.

- **Protocols:** VLESS (Reality & TLS), VMess, Trojan, Shadowsocks — natively via
  xray-core; **Hysteria2** — via a separate engine on the node (sing-box), because
  Hysteria2 (QUIC) is not an xray-core inbound.
- **Access model:** a client is granted specific *inbounds* (not whole servers),
  many-to-many. Its **subscription is assembled automatically** from all allowed &
  enabled inbounds across every server and served at one public URL: `GET /sub/{token}`.
- **Stack:** Go backend · React + TypeScript frontend · SQLite · Caddy (HTTPS) ·
  Docker Compose.

> Secrets (SSH keys/passwords, TOTP secret) are **encrypted at rest** (AES-256-GCM);
> the admin password is only a bcrypt hash. Nothing is hardcoded — all secrets come
> from the environment.

## Requirements

- Docker + Docker Compose (locally we use **Colima** on macOS).
- Managed nodes must be **Debian/Ubuntu** with root/sudo SSH and outbound internet
  (the panel auto-installs xray-core + sing-box on server add).

## Quick start (local, full stack)

1. Create `.env` from the example and fill in the secrets:

   ```bash
   cp .env.example .env
   # PANEL_ENCRYPTION_KEY — 32 bytes, base64:  openssl rand -base64 32
   # PANEL_JWT_SECRET     — any long random string
   # PANEL_ADMIN_USERNAME / PANEL_ADMIN_PASSWORD — first admin login
   # PANEL_DOMAIN         — localhost for local; your domain on the VPS
   ```

2. Bring up the whole panel:

   ```bash
   docker compose up --build
   ```

3. Open **https://localhost** (accept the local-CA warning — Caddy issues a
   self-signed cert locally). Log in with the admin credentials; enable 2FA in
   Settings. API docs (Swagger): **https://localhost/swagger**.

4. Smoke-test the running stack end to end:

   ```bash
   ./scripts/smoke.sh
   ```

   It logs in, creates a server + inbounds (incl. Hysteria2), a client, grants
   access, and asserts `GET /sub/{token}` returns a non-empty subscription with
   both `vless://` and `hysteria2://` URIs.

## Development

Backend (from `backend/`):

```bash
go run ./cmd/panel     # needs the env vars from .env
go test ./...          # unit + e2e tests
go vet ./... && gofmt -l .
# engine-integration tests (need Docker):
go test -tags=integration -run Integration ./internal/protocols/ ./internal/sync/ -v
```

Frontend (from `frontend/`):

```bash
npm ci
npm run dev            # http://localhost:5173 — runs on mock data by default
npm run build          # production build (mocks off)
npm run test && npm run lint
```

- The frontend talks to the backend only through the REST API + generated types
  (`src/api/schema.ts`, regenerate with `npm run gen:api`). The mock layer (MSW)
  and the real backend both honor `docs/openapi.yaml`.
- Mocks are controlled by `VITE_USE_MOCKS` (defaults on for `npm run dev`; the
  Docker image builds with `VITE_USE_MOCKS=false`). To run dev against a live
  backend: `VITE_USE_MOCKS=false npm run dev` with the backend on `:8080`.

## Environment variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `PANEL_ENCRYPTION_KEY` | ✅ | AES-256 master key, base64 of 32 bytes. Encrypts SSH/TOTP secrets. |
| `PANEL_JWT_SECRET` | ✅ | Signs session tokens. |
| `PANEL_ADMIN_USERNAME` | | First admin username (default `admin`). |
| `PANEL_ADMIN_PASSWORD` | ✅ | First admin password (bcrypt-hashed on seed). |
| `PANEL_DOMAIN` | | Public hostname (default `localhost`). |
| `PANEL_SUB_BASE_URL` | | Base URL for subscription links (derived from domain if unset). |
| `DB_PATH` | | SQLite path (default `/data/panel.db`). |
| `PANEL_HTTP_ADDR` | | Backend listen address (default `:8080`). |
| `PANEL_SITE_ADDRESS` | | Caddy site (compose): `localhost` locally, your domain on the VPS (auto Let's Encrypt). |

A missing **required** variable is a fatal startup error — the panel never invents
secrets.

## How it works

- **Provisioning** (`internal/provision`): on server add, the panel installs
  xray-core + sing-box as systemd services and generates a self-signed TLS cert for
  Hysteria2. Debian/Ubuntu only. Until a node is `installed`, sync won't push to it.
- **Sync** (`internal/sync`) is **multi-engine**: it assembles each engine's config
  (Xray `config.json` and, for Hysteria2 inbounds, the sing-box config) from a node's
  enabled inbounds and their granted clients, then over SSH: backs up, writes,
  validates (`xray -test` / `sing-box check`), restores the backup on failure, and
  restarts the service. Idempotent by config hash.
- **Hysteria2** is a QUIC protocol and is **not** an xray-core inbound: it never
  enters Xray's `config.json`. It compiles to a separate sing-box config on the node
  and produces `hysteria2://…` subscription URIs. See `docs/SPEC.md §5`.

### Adding a protocol

Protocols live in a registry (`backend/internal/protocols`). To add one, implement
the `Protocol` interface (`Name`, `Engine`, `BuildInbound`, `BuildLink`) and register
it in the adapter's `init()`. **Do not change the DB schema** — protocol-specific
parameters live in the JSON columns (`settings`, `stream_settings`, `sniffing`).
Wanting a new column is a design smell.

## Troubleshooting

- **Browser TLS warning at https://localhost** — expected: Caddy uses a local CA.
  Accept it, or trust Caddy's root cert.
- **`docker compose` can't find the daemon (macOS)** — start Colima: `colima start`.
- **Server stuck `provisioning` / sync refuses to push** — the node must reach
  `installed` first (Debian/Ubuntu, root SSH, outbound internet). Re-run install from
  the server card.
- **Config rejected on sync** — the panel validates with `xray -test` / `sing-box
  check` and restores the previous config; the reason is stored in the server's
  `last_sync_error`.

## Project layout

```
backend/    Go API, protocol registry, provisioning, sync engine
frontend/   React + TS SPA (mock layer + real API client, same contract)
deploy/      Caddyfile, web image (frontend build + Caddy)
docs/        SPEC, ROADMAP, ARCHITECTURE, openapi.yaml, PROGRESS
scripts/     smoke.sh
test/        dockerized xray / hysteria nodes for integration tests
```

Full spec: `docs/SPEC.md`. Phased plan & status: `docs/ROADMAP.md`, `docs/PROGRESS.md`.
