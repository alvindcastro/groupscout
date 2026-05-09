#!/usr/bin/env bash
set -euo pipefail

GROUPSCOUT_REPO="${GROUPSCOUT_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GROUPSCOUT_UI_REPO="${GROUPSCOUT_UI_REPO:-/mnt/c/Users/alvin/WebstormProjects/groupscout-ui}"
GROUPSCOUT_UI_HOST_PORT="${GROUPSCOUT_UI_HOST_PORT:-3001}"
GROUPSCOUT_UI_PROD_PORT="${GROUPSCOUT_UI_PROD_PORT:-3002}"
GROUPSCOUT_UI_BAD_PORT="${GROUPSCOUT_UI_BAD_PORT:-3003}"
PROD_CONTAINER="groupscout-ui-production-smoke"
BAD_CONTAINER="groupscout-ui-production-bad-proxy-smoke"

cleanup() {
  docker stop "$PROD_CONTAINER" >/dev/null 2>&1 || true
  docker stop "$BAD_CONTAINER" >/dev/null 2>&1 || true
  docker compose -p groupscout \
    -f "$GROUPSCOUT_REPO/docker-compose.yml" \
    -f "$GROUPSCOUT_UI_REPO/compose.dev.yml" \
    down >/dev/null 2>&1 || true
}
trap cleanup EXIT

require_file() {
  if [[ ! -e "$1" ]]; then
    echo "missing required path: $1" >&2
    exit 1
  fi
}

expect_status() {
  local url="$1"
  local want="$2"
  local label="$3"
  local got
  got="$(curl -sS -o /tmp/groupscout-smoke-body -w '%{http_code}' "$url" || true)"
  if [[ "$got" != "$want" ]]; then
    echo "$label: expected HTTP $want, got $got" >&2
    cat /tmp/groupscout-smoke-body >&2 || true
    exit 1
  fi
  echo "ok: $label returned $want"
}

scan_no_secrets() {
  local path="$1"
  require_file "$path"
  if grep -RInE 'API_TOKEN|DATABASE_URL|POSTGRES_URL|SLACK_WEBHOOK_URL|RESEND_API_KEY|SENDGRID_API_KEY|OPENAI_API_KEY|ANTHROPIC_API_KEY|CLAUDE_API_KEY|OLLAMA_BASE_URL|UI_SESSION_SECRET|Authorization|Bearer|-----BEGIN .*PRIVATE KEY-----' "$path"; then
    echo "static asset secret scan failed for $path" >&2
    exit 1
  fi
}

require_file "$GROUPSCOUT_UI_REPO/compose.dev.yml"

docker compose -p groupscout \
  -f "$GROUPSCOUT_REPO/docker-compose.yml" \
  -f "$GROUPSCOUT_UI_REPO/compose.dev.yml" \
  up -d --build groupscout groupscout-ui

expect_status "http://localhost:8080/health" "200" "backend health"
expect_status "http://localhost:${GROUPSCOUT_UI_HOST_PORT}/healthz" "200" "UI D3 healthz"

docker build --target production -t groupscout-ui-production "$GROUPSCOUT_UI_REPO"
docker run --rm -d \
  --name "$PROD_CONTAINER" \
  --network groupscout_groupscout_net \
  -p "${GROUPSCOUT_UI_PROD_PORT}:3000" \
  -e UI_API_PROXY_TARGET=http://groupscout:8080 \
  groupscout-ui-production >/dev/null

expect_status "http://localhost:${GROUPSCOUT_UI_PROD_PORT}/healthz" "200" "production UI healthz"
expect_status "http://localhost:${GROUPSCOUT_UI_PROD_PORT}/" "200" "production UI root"
expect_status "http://localhost:${GROUPSCOUT_UI_PROD_PORT}/assets/app.js" "200" "production UI static asset"
expect_status "http://localhost:${GROUPSCOUT_UI_PROD_PORT}/api/leads?limit=1" "200" "same-origin /api/leads proxy"

system_status="$(curl -sS -o /tmp/groupscout-smoke-system -w '%{http_code}' "http://localhost:${GROUPSCOUT_UI_PROD_PORT}/api/system" || true)"
if [[ "$system_status" == "404" ]]; then
  echo "ok: backend 404 reached through proxy for /api/system"
elif [[ "$system_status" == "200" ]]; then
  echo "ok: backend /api/system is implemented and reached through proxy"
else
  echo "unexpected /api/system status through good proxy: $system_status" >&2
  cat /tmp/groupscout-smoke-system >&2 || true
  exit 1
fi

docker run --rm -d \
  --name "$BAD_CONTAINER" \
  --network groupscout_groupscout_net \
  -p "${GROUPSCOUT_UI_BAD_PORT}:3000" \
  -e UI_API_PROXY_TARGET=http://groupscout-missing:8080 \
  groupscout-ui-production >/dev/null
expect_status "http://localhost:${GROUPSCOUT_UI_BAD_PORT}/api/system" "502" "proxy 502 bad target"

(
  cd "$GROUPSCOUT_UI_REPO"
  node --test test/app-shell.test.js test/lead-inbox-screen.test.js test/lead-detail-screen.test.js
)

scan_no_secrets "$GROUPSCOUT_REPO/plugins/groupscout-agents/assets"
scan_no_secrets "$GROUPSCOUT_UI_REPO/web/dist"
if grep -RIn 'http://groupscout:8080' "$GROUPSCOUT_UI_REPO/web/dist"; then
  echo "browser assets must use relative /api/*, not http://groupscout:8080" >&2
  exit 1
fi

echo "Phase 38 UI Docker smoke passed"

