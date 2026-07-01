#!/usr/bin/env bash
# End-to-end smoke test (SPEC §12): health → login → server → inbounds (incl.
# Hysteria2) → client → grant → non-empty subscription.
#
# Runs against the backend API directly (default http://localhost:8080), so it
# works with `docker compose up` or a local `go run ./cmd/panel`.
#
#   BASE=http://localhost:8080 ADMIN_USER=admin ADMIN_PASS=... ./scripts/smoke.sh
#
# Admin credentials default to PANEL_ADMIN_USERNAME/PANEL_ADMIN_PASSWORD from a
# local .env if present. Requires: bash, curl, python3.
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"

# Load admin creds from .env when not passed explicitly.
if [[ -f .env ]]; then
  # shellcheck disable=SC1091
  set -a; source .env; set +a
fi
ADMIN_USER="${ADMIN_USER:-${PANEL_ADMIN_USERNAME:-admin}}"
ADMIN_PASS="${ADMIN_PASS:-${PANEL_ADMIN_PASSWORD:-}}"

if [[ -z "$ADMIN_PASS" ]]; then
  echo "FAIL: no admin password (set ADMIN_PASS or PANEL_ADMIN_PASSWORD)" >&2
  exit 1
fi

# jget <json> <python-expression-on-d> — extract a field via python3.
jget() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$2" <<<"$1"; }

step() { printf "\n\033[1m▶ %s\033[0m\n" "$1"; }

step "Waiting for $BASE/healthz"
for i in $(seq 1 30); do
  if curl -fsS "$BASE/healthz" >/dev/null 2>&1; then break; fi
  sleep 1
  if [[ "$i" == 30 ]]; then echo "FAIL: backend not healthy" >&2; exit 1; fi
done
echo "  healthy"

step "Login as $ADMIN_USER"
LOGIN=$(curl -fsS -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")
NEED_2FA=$(jget "$LOGIN" "d.get('need_2fa')")
if [[ "$NEED_2FA" == "True" ]]; then
  echo "FAIL: admin has 2FA enabled — smoke expects a fresh admin" >&2
  exit 1
fi
TOKEN=$(jget "$LOGIN" "d['tokens']['token']")
echo "  token acquired"
AUTH=(-H "Authorization: Bearer $TOKEN")

step "Create server"
SRV=$(curl -fsS -X POST "$BASE/api/servers" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d '{"name":"smoke-node","host":"203.0.113.10","ssh_user":"root","ssh_auth_method":"password","ssh_secret":"x"}')
SID=$(jget "$SRV" "d['id']")
echo "  server id=$SID"

step "Create VLESS + Hysteria2 inbounds"
curl -fsS -X POST "$BASE/api/servers/$SID/inbounds" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d '{"tag":"vless-tcp","protocol":"vless","port":443,"settings":{"decryption":"none"},"stream_settings":{"network":"tcp","security":"none"}}' >/dev/null
HY=$(curl -fsS -X POST "$BASE/api/servers/$SID/inbounds" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d '{"tag":"hy2","protocol":"hysteria2","port":36712,"settings":{"up":"100 mbps","down":"200 mbps"}}')
echo "  inbounds created"

step "Create client and grant both inbounds"
CL=$(curl -fsS -X POST "$BASE/api/clients" "${AUTH[@]}" -H 'Content-Type: application/json' -d '{"name":"smoke-user"}')
CID=$(jget "$CL" "d['id']")
TOKEN_SUB=$(jget "$CL" "d['subscription_token']")
IDS=$(curl -fsS "$BASE/api/servers/$SID/inbounds" "${AUTH[@]}" | python3 -c 'import sys,json; print(json.dumps([i["id"] for i in json.load(sys.stdin)]))')
curl -fsS -X PUT "$BASE/api/clients/$CID/inbounds" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d "{\"inbound_ids\":$IDS}" >/dev/null
echo "  client id=$CID granted inbounds $IDS"

step "Fetch subscription /sub/$TOKEN_SUB"
SUB=$(curl -fsS "$BASE/sub/$TOKEN_SUB")
DECODED=$(python3 -c 'import sys,base64; print(base64.b64decode(sys.stdin.read()).decode())' <<<"$SUB")
COUNT=$(printf '%s\n' "$DECODED" | grep -c '://' || true)
echo "  decoded $COUNT URI(s):"
printf '%s\n' "$DECODED" | sed 's/^/    /'

if [[ "$COUNT" -lt 2 ]]; then
  echo "FAIL: expected >=2 URIs (vless + hysteria2), got $COUNT" >&2
  exit 1
fi
if ! grep -q '^vless://' <<<"$DECODED" || ! grep -q '^hysteria2://' <<<"$DECODED"; then
  echo "FAIL: subscription missing a scheme" >&2
  exit 1
fi

printf "\n\033[1;32m✓ SMOKE PASSED\033[0m — subscription has vless:// + hysteria2://\n"
