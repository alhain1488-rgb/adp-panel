# Absolutely Disgusting Panel

[Русский](README.md) · **English**

A self-hosted web panel to manage several VPS running **Xray**, **Hysteria2** and
**AmneziaWG** proxies from one place — for **personal use**. One panel on a main VPS controls the
proxies on all your nodes over SSH: servers, inbounds of many protocols, clients, and their
subscriptions. No billing, traffic limits, or multi-tenancy — by design.

- **Protocols:** VLESS (Reality & TLS), VMess, Trojan, Shadowsocks — natively via xray-core;
  **Hysteria2** — via a separate engine on the node (sing-box), because Hysteria2 (QUIC) is not an
  xray-core inbound; **AmneziaWG 2.0** — obfuscated WireGuard, its own engine on the node.
- **Access model:** a client is granted specific *inbounds* (not whole servers), many-to-many. Its
  **subscription is assembled automatically** from all allowed & enabled inbounds across every
  server and served at one public URL: `GET /sub/{token}`.
- **Stack:** Go backend · React + TypeScript frontend · SQLite · Caddy (HTTPS) · Docker Compose.

> Secrets (SSH keys/passwords, TOTP secret, Resend key, passphrases) are **encrypted at rest**
> (AES-256-GCM); the admin password is only a bcrypt hash. Nothing is hardcoded — all secrets come
> from the environment or are set in the UI and encrypted.

---

## Features

**Core**
- Manage several **nodes** over SSH: status, geo, engine versions, auto-provisioning.
- **Inbounds** of every protocol above; protocol-specific params live in JSON columns (the DB schema
  never changes when adding a protocol).
- **Clients** with generated credentials (UUID, proxy password, subscription token), per-inbound
  access grants, enable/disable, token rotation.
- **Subscription** assembled automatically: `vless://`, `vmess://`, `trojan://`, `ss://`,
  `hysteria2://` and (separately) AmneziaWG configs.
- **Multi-engine sync**: xray-core, sing-box and AmneziaWG run side by side on one node.

**Client delivery**
- **Client portal** `/portal` — a public page: a client signs in with name + subscription token and
  sees a read-only view of their subscription (link + QR, per-server configs, AmneziaWG).
- **Telegram bot**: a client links their chat via a personal deep link and instantly gets their
  config; afterwards they fetch it themselves with `/config` or a button. Plus a **“Send configs”**
  button that broadcasts every client’s link + QR to the backup Telegram and e-mail.
- **E-mail**: send a client their subscription (via **Resend** over HTTPS, or plain SMTP).
- All messages to clients (Telegram and e-mail) are **bilingual RU (EN)** with a branding plate.

**Operations**
- **Backups** three ways: manual export/import of an encrypted archive, auto-backup to **Telegram**,
  auto-backup to **e-mail** (scheduled).
- A **System** page — live host load of the panel machine (CPU, memory, disk, uptime).
- **i18n**: Russian (default) and English; dark/light theme.
- **Security**: encrypted secrets, bcrypt admin password, **2FA (TOTP)**, audit log, rate-limiting on
  public endpoints.

---

## Install on a VPS (one command)

On a fresh **Debian/Ubuntu** server, `install.sh` does everything: installs Docker, adds swap on
small boxes, generates secrets, picks a TLS site address, builds the images, and starts the stack.
Re-running it updates in place and keeps your secrets.

```bash
# as root, from a checkout of this repo:
git clone https://github.com/alhain1488-rgb/adp-panel.git /opt/adp-panel
cd /opt/adp-panel && sudo bash install.sh
```

- **No domain?** Leave it — the panel is served on `<your-ip>.sslip.io` with a **trusted Let's
  Encrypt cert** (no domain purchase needed).
- **Have a domain** pointing at the server? Pass it for a cert on your own name:

  ```bash
  sudo PANEL_DOMAIN=vpn.example.com bash install.sh
  ```

The installer prints the URL and admin credentials at the end (also saved in `/opt/adp-panel/.env`).
Options: `PANEL_ADMIN_USERNAME`, `PANEL_ADMIN_PASSWORD`, `INSTALL_DIR`, `REPO_URL`/`REPO_BRANCH`,
`GITHUB_TOKEN` (for a private repo).

### Update / redeploy

```bash
cd /opt/adp-panel && git pull && docker compose up --build -d
docker image prune -f && docker builder prune -f   # keep the disk from creeping up
```

Caddy renews the Let's Encrypt cert automatically. If the server’s IP changes, only update the
domain’s A record to the new IP — client subscriptions point at the domain, not the IP.

---

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

3. Open **https://localhost** (accept the local-CA warning — Caddy issues a self-signed cert
   locally). Log in with the admin credentials; enable 2FA in Settings. API docs (Swagger):
   **https://localhost/swagger**.

4. Smoke-test the running stack end to end:

   ```bash
   ./scripts/smoke.sh
   ```

---

## Environment variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `PANEL_ENCRYPTION_KEY` | ✅ | AES-256 master key, base64 of 32 bytes. Encrypts SSH/TOTP/mail secrets. |
| `PANEL_JWT_SECRET` | ✅ | Signs session tokens. |
| `PANEL_ADMIN_USERNAME` | | First admin username (default `admin`). |
| `PANEL_ADMIN_PASSWORD` | ✅ | First admin password (bcrypt-hashed on seed). |
| `PANEL_DOMAIN` | | Public hostname (default `localhost`). |
| `PANEL_SUB_BASE_URL` | | Base URL for subscription links (derived from domain if unset). |
| `DB_PATH` | | SQLite path (default `/data/panel.db`). |
| `PANEL_HTTP_ADDR` | | Backend listen address (default `:8080`). |
| `PANEL_SITE_ADDRESS` | | Caddy site (compose): `localhost` locally, your domain on the VPS (auto Let's Encrypt). Comma-separate several. |

A missing **required** variable is a fatal startup error — the panel never invents secrets.

> E-mail, the Telegram bot and backups are configured **in the UI** (Settings → Backups / E-mail
> delivery); their secrets (bot token, Resend key, passphrases) are encrypted in the DB with the same
> master key.

---

## How it works

### Provisioning (`internal/provision`)
On server add, the panel installs engines as systemd services over SSH: **xray-core** + **sing-box**
(for Hysteria2), and **AmneziaWG** when needed (kernel module via DKMS from `ppa:amnezia/ppa`, or
userspace `amneziawg-go`), and generates a self-signed TLS cert for Hysteria2. Debian/Ubuntu only.
Until a node is `installed`, sync won’t push to it.

### Multi-engine sync (`internal/sync`)
Sync spreads a node’s inbounds across engines, builds **each** engine’s config from its enabled
inbounds and granted clients, then over SSH: backs up, writes, **validates** (`xray -test` for Xray,
config check for sing-box/AmneziaWG), restores the backup on failure and restarts the service.
Idempotent by config hash.

### Hysteria2 and AmneziaWG — why separate engines
- **Hysteria2** is a QUIC protocol and is **not** an xray-core inbound: it never enters Xray’s
  `config.json`. It compiles to a separate sing-box config and yields `hysteria2://…` in the
  subscription.
- **AmneziaWG 2.0** is obfuscated WireGuard, run as its own service (`awg-quick@awgN`). A client gets
  two formats: a **`vpn://`** link for the AmneziaVPN app (compatible with its 2.0 format) and a
  **`.conf`** file for the AmneziaWG app. Obfuscation (Jc/Jmin/Jmax, S1–S4, H1–H4, I1) follows the
  AmneziaVPN defaults.

### Subscription
The only public (token-addressed) endpoint `GET /sub/{token}` assembles the subscription from all
allowed and enabled inbounds of the client across every server. The client pastes the link into their
app (sing-box, Hiddify, v2rayN, Streisand…) and gets all configs automatically.

### Client portal (`/portal`)
A public page outside the admin login. A client enters **name + subscription token** (a full `/sub/…`
link is also accepted — the token is extracted) → the server looks up the client by token, verifies
the name case-insensitively, admits enabled clients only, and returns a read-only subscription: link
+ QR, per-server configs (copy + QR), AmneziaWG. Rate-limited with a generic error (no oracle). Login
= name, password = the existing token; there is no separate password system.

### Telegram bot
One bot token serves both backups and client delivery. The Bot API **won’t let a bot message a user
first**, so:
1. The panel gives the client a personal link `t.me/<bot>?start=<token>`.
2. The client opens it and presses **Start** → a background `getUpdates` poller catches
   `/start <token>`, stores the client’s `chat_id` and **sends the config immediately**.
3. Afterwards the client fetches it themselves with the “🔄 Get config” button or `/config`.

The **“Send configs”** button on the Clients tab broadcasts every client’s link + QR to the backup
Telegram (monospace = tap to copy) and the backup e-mail.

### E-mail
Provider **Resend** (over HTTPS, port 443) or plain SMTP. Resend is needed where the host blocks
outbound SMTP. A client’s subscription e-mail is sent as two parts — plain text and HTML with a
branding plate (the logo loads from `/cheremsha.png`). Copy is bilingual — Russian with an English
translation in parentheses.

### Backups
- **Export/import**: an encrypted `.adpbak` archive (AES-256-GCM with a passphrase). Import is applied
  on the next boot (the panel restarts).
- **Auto-backup to Telegram** and **to e-mail**: on a schedule (interval in hours), recording the last
  run’s status.

### System page
`GET /api/system` reads host metrics from `/proc` (`loadavg`, `meminfo`, `uptime`) and `statfs`. On
Linux these reflect the **host** even when the backend runs in a container, so the numbers describe
the VPS itself. The page refreshes every 4s.

### Adding a protocol
Protocols live in a registry (`backend/internal/protocols`). To add one, implement the `Protocol`
interface (`Name`, `Engine`, `BuildInbound`, `BuildLink`) and register it in the adapter’s `init()`.
**Do not change the DB schema** — protocol-specific parameters live in the JSON columns (`settings`,
`stream_settings`, `sniffing`). Wanting a new column is a design smell.

---

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

- The frontend talks to the backend only through the REST API + generated types (`src/api/schema.ts`,
  regenerate with `npm run gen:api`). The mock layer (MSW) and the real backend both honor
  `docs/openapi.yaml`.
- Mocks are controlled by `VITE_USE_MOCKS` (defaults on for `npm run dev`; the Docker image builds
  with `VITE_USE_MOCKS=false`). To run dev against a live backend: `VITE_USE_MOCKS=false npm run dev`.

---

## Troubleshooting

- **Browser TLS warning at https://localhost** — expected: Caddy uses a local CA. Accept it, or trust
  Caddy’s root cert.
- **`docker compose` can’t find the daemon (macOS)** — start Colima: `colima start`.
- **Server stuck `provisioning` / sync refuses to push** — the node must reach `installed` first
  (Debian/Ubuntu, root SSH, outbound internet). Re-run install from the server card.
- **Config rejected on sync** — the panel validates with `xray -test` / a config check and restores
  the previous config; the reason is stored in the server’s `last_sync_error`.
- **E-mail won’t send over SMTP** — many hosts block outbound SMTP (25/465/587). Switch the mail
  provider to **Resend** (Settings → E-mail delivery).
- **The mascot doesn’t appear next to bot messages** — set the bot’s avatar once in @BotFather:
  `/setuserpic` → pick the bot → send `https://<your-domain>/cheremsha.png`.

---

РАФОН - ЛОХ

## Project layout

```
backend/    Go API, protocol registry, provisioning, sync engine, mail, backups, Telegram bot
frontend/   React + TS SPA (mock layer + real API client, same contract), portal, i18n
deploy/     Caddyfile, web image (frontend build + Caddy)
docs/       SPEC, ROADMAP, ARCHITECTURE, openapi.yaml, PROGRESS
scripts/    smoke.sh
test/       dockerized xray / hysteria nodes for integration tests
```

Full spec: `docs/SPEC.md`. Phased plan & status: `docs/ROADMAP.md`, `docs/PROGRESS.md`.
