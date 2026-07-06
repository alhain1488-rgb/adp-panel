#!/usr/bin/env bash
#
# Absolutely Disgusting Panel — one-command installer for a fresh Debian/Ubuntu VPS.
#
# From a checkout:
#     sudo bash install.sh
#
# Standalone (repo must be reachable — public, or GITHUB_TOKEN set):
#     curl -fsSL <raw-url>/install.sh | sudo -E bash
#
# It installs Docker, adds swap on small boxes, generates secrets, picks a TLS
# site address, builds the images and starts the stack. Re-running updates in
# place and keeps existing secrets.
#
# Configuration (all optional, via environment):
#   PANEL_DOMAIN          Real domain pointing at this server → automatic Let's
#                         Encrypt cert. If unset, uses <public-ip>.sslip.io so
#                         HTTPS works with a trusted cert even without a domain.
#   PANEL_ADMIN_USERNAME  Admin login (default: admin).
#   PANEL_ADMIN_PASSWORD  Admin password (default: generated and printed).
#   INSTALL_DIR           Where to place the project (default: /opt/adp-panel).
#   REPO_URL              Git URL to clone when not run from a checkout.
#   REPO_BRANCH           Branch to clone (default: build/mvp).
#   GITHUB_TOKEN          Token for cloning a private repo over HTTPS.
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/adp-panel}"
REPO_URL="${REPO_URL:-https://github.com/alhain1488-rgb/adp-panel.git}"
REPO_BRANCH="${REPO_BRANCH:-build/mvp}"

log()  { printf '\033[1;32m▶ %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$*" >&2; }
die()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Please run as root (sudo bash install.sh)."

export DEBIAN_FRONTEND=noninteractive TERM="${TERM:-xterm}"

# --- OS check -------------------------------------------------------------
# shellcheck disable=SC1091
. /etc/os-release 2>/dev/null || true
case "${ID:-}" in
  debian | ubuntu) : ;;
  *) warn "Untested OS '${ID:-unknown}'. This installer targets Debian/Ubuntu." ;;
esac

# --- base packages --------------------------------------------------------
if ! command -v git >/dev/null || ! command -v curl >/dev/null || ! command -v openssl >/dev/null; then
  log "Installing base packages (git, curl, openssl)…"
  apt-get update -y
  apt-get install -y --no-install-recommends git curl ca-certificates openssl
fi

# --- Docker ---------------------------------------------------------------
if ! command -v docker >/dev/null; then
  log "Installing Docker…"
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker >/dev/null 2>&1 || true
fi
docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is missing."

# --- swap (image builds need memory on tiny VPS) --------------------------
mem_mb="$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)"
if [ "${mem_mb:-0}" -lt 1800 ] && ! swapon --show | grep -q .; then
  log "Low RAM (${mem_mb}MB) — adding 2G swap for the build…"
  fallocate -l 2G /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
fi

# --- obtain the project ---------------------------------------------------
if [ -f "docker-compose.yml" ] && [ -d "backend" ]; then
  APP_DIR="$(pwd)"
elif [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
  log "Updating existing install in $INSTALL_DIR…"
  git -C "$INSTALL_DIR" pull --ff-only 2>/dev/null || warn "git pull skipped; using existing files."
  APP_DIR="$INSTALL_DIR"
else
  clone_url="$REPO_URL"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    clone_url="https://x-access-token:${GITHUB_TOKEN}@${REPO_URL#https://}"
  fi
  log "Cloning into $INSTALL_DIR…"
  git clone --branch "$REPO_BRANCH" --depth 1 "$clone_url" "$INSTALL_DIR" ||
    die "Clone failed. For a private repo, set GITHUB_TOKEN or run install.sh from a checkout."
  APP_DIR="$INSTALL_DIR"
fi
cd "$APP_DIR"

# --- TLS site address -----------------------------------------------------
if [ -n "${PANEL_DOMAIN:-}" ]; then
  SITE="$PANEL_DOMAIN"
elif [ -f .env ] && grep -q '^PANEL_SITE_ADDRESS=' .env; then
  SITE="$(grep '^PANEL_SITE_ADDRESS=' .env | cut -d= -f2-)"
else
  ip="$(curl -fsS4 https://api.ipify.org 2>/dev/null || curl -fsS https://ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')"
  [ -n "$ip" ] || die "Could not detect a public IP. Set PANEL_DOMAIN explicitly."
  SITE="${ip}.sslip.io"
  log "No domain given — serving on ${SITE} (trusted TLS via sslip.io, no domain needed)."
fi

# --- .env (generate once; keep secrets on re-run) -------------------------
if [ ! -f .env ]; then
  log "Generating .env with fresh secrets…"
  admin_user="${PANEL_ADMIN_USERNAME:-admin}"
  admin_pass="${PANEL_ADMIN_PASSWORD:-$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-16)}"
  umask 077
  cat >.env <<EOF
PANEL_ENCRYPTION_KEY=$(openssl rand -base64 32)
PANEL_JWT_SECRET=$(openssl rand -base64 48)
PANEL_ADMIN_USERNAME=${admin_user}
PANEL_ADMIN_PASSWORD=${admin_pass}
PANEL_DOMAIN=${SITE}
PANEL_SUB_BASE_URL=https://${SITE}
PANEL_SITE_ADDRESS=${SITE}
DB_PATH=/data/panel.db
PANEL_HTTP_ADDR=:8080
EOF
  chmod 600 .env
else
  log "Existing .env found — keeping secrets; site address = ${SITE}."
  sed -i -E \
    -e "s|^PANEL_DOMAIN=.*|PANEL_DOMAIN=${SITE}|" \
    -e "s|^PANEL_SITE_ADDRESS=.*|PANEL_SITE_ADDRESS=${SITE}|" \
    -e "s|^PANEL_SUB_BASE_URL=.*|PANEL_SUB_BASE_URL=https://${SITE}|" \
    .env
fi

# --- build & start --------------------------------------------------------
log "Building and starting the panel (first run can take a few minutes)…"
docker compose up -d --build

log "Waiting for the backend to become healthy…"
for _ in $(seq 1 60); do
  if docker compose exec -T backend wget -qO- http://localhost:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 3
done

# --- reclaim disk ---------------------------------------------------------
# Each rebuild leaves a superseded image + build cache behind; on a small VPS
# that piles up fast, so prune the leftovers now that the new stack is running.
log "Reclaiming disk from old images and build cache…"
docker image prune -f >/dev/null 2>&1 || true
docker builder prune -f >/dev/null 2>&1 || true

# --- summary --------------------------------------------------------------
admin_user="$(grep -E '^PANEL_ADMIN_USERNAME=' .env | cut -d= -f2-)"
admin_pass="$(grep -E '^PANEL_ADMIN_PASSWORD=' .env | cut -d= -f2-)"
printf '\n\033[1;32m✅ Absolutely Disgusting Panel is up.\033[0m\n\n'
cat <<EOF
    URL:       https://${SITE}
    Login:     ${admin_user}
    Password:  ${admin_pass}

    Secrets live in ${APP_DIR}/.env (chmod 600) — back them up.
    Manage:    cd ${APP_DIR} && docker compose ps | logs -f | restart | down
    Update:    re-run this installer (keeps your secrets and data).

    Next, in the panel → Settings: e-mail (Resend/SMTP), Telegram bot and
    auto-backups. Clients can self-serve their subscription at https://${SITE}/portal
EOF

if [ -z "${PANEL_DOMAIN:-}" ]; then
  printf '\n\033[1;33mNote:\033[0m sslip.io needs public DNS to reach it — some local resolvers with\n'
  printf 'DNS-rebind protection mangle *.sslip.io. If it will not open, try mobile data,\n'
  printf 'switch the client DNS to 1.1.1.1, or point a real domain at this IP and re-run\n'
  printf 'with PANEL_DOMAIN=your.domain.\n'
fi
