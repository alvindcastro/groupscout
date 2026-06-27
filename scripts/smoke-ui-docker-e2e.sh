#!/usr/bin/env bash
# Phase 38 backend-owned UI container E2E smoke gate.
# Verifies the backend Compose stack plus the external UI production container.
# The production UI Compose profile lives in the UI repo.
set -euo pipefail

GROUPSCOUT_UI_REPO="${GROUPSCOUT_UI_REPO:-/mnt/c/Users/alvin/WebstormProjects/groupscout-ui}"
GROUPSCOUT_BACKEND_REPO="${GROUPSCOUT_BACKEND_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GROUPSCOUT_COMPOSE="${GROUPSCOUT_COMPOSE:-docker compose}"
BACKEND_PORT="${GROUPSCOUT_BACKEND_HOST_PORT:-8080}"
UI_PORT="${GROUPSCOUT_UI_PRODUCTION_HOST_PORT:-3002}"
BAD_PROXY_PORT="${GROUPSCOUT_UI_BAD_PROXY_HOST_PORT:-3003}"

log() { echo "[smoke-ui-docker-e2e] $*"; }
fail() { echo "[smoke-ui-docker-e2e] FAIL: $*" >&2; exit 1; }

read -r -a COMPOSE <<< "${GROUPSCOUT_COMPOSE}"
COMPOSE_FILES=(
    -f "${GROUPSCOUT_BACKEND_REPO}/docker-compose.yml"
    -f "${GROUPSCOUT_UI_REPO}/compose.dev.yml"
)
COMPOSE_ARGS=(
    -p groupscout
    "${COMPOSE_FILES[@]}"
    --profile smoke-ui-e2e
)

# Cleanup removes only the external UI smoke containers and preserves backend data volumes.
cleanup() {
    log "cleanup: stopping UI Compose containers..."
    "${COMPOSE[@]}" "${COMPOSE_ARGS[@]}" \
        stop groupscout-ui-production groupscout-ui-production-bad-proxy 2>/dev/null || true
    "${COMPOSE[@]}" "${COMPOSE_ARGS[@]}" \
        rm -f groupscout-ui-production groupscout-ui-production-bad-proxy 2>/dev/null || true
    log "cleanup: done"
}
trap cleanup EXIT

# 1. Backend health
log "checking backend health at :${BACKEND_PORT}/health"
curl -sf "http://localhost:${BACKEND_PORT}/health" >/dev/null \
    || fail "backend not healthy at :${BACKEND_PORT}/health - start the backend Compose stack first"

# 2. Validate UI Compose config
log "validating UI Compose profile smoke-ui-e2e"
"${COMPOSE[@]}" "${COMPOSE_ARGS[@]}" \
    config --quiet \
    || fail "UI Compose config invalid in ${GROUPSCOUT_UI_REPO}"

# 3. Start UI production containers
log "starting UI production containers (profile smoke-ui-e2e)"
"${COMPOSE[@]}" "${COMPOSE_ARGS[@]}" \
    up -d --build groupscout-ui-production groupscout-ui-production-bad-proxy

# 4. Secret scan on UI Compose overlay
PATTERNS=("API_TOKEN" "DATABASE_URL" "SLACK_WEBHOOK_URL" "RESEND_API_KEY" "CLAUDE_API_KEY" "OLLAMA_ENDPOINT" "UI_SESSION_SECRET")
COMPOSE_OVERLAY=""
for candidate in \
    "${GROUPSCOUT_UI_REPO}/docker-compose.smoke-ui-e2e.yml" \
    "${GROUPSCOUT_UI_REPO}/compose.smoke-ui-e2e.yml" \
    "${GROUPSCOUT_UI_REPO}/docker-compose.yml" \
    "${GROUPSCOUT_UI_REPO}/compose.dev.yml"; do
    if [[ -f "${candidate}" ]]; then
        COMPOSE_OVERLAY="${candidate}"
        break
    fi
done

if [[ -n "${COMPOSE_OVERLAY}" ]]; then
    log "scanning ${COMPOSE_OVERLAY} for leaked secrets"
    for pat in "${PATTERNS[@]}"; do
        if grep -q "${pat}" "${COMPOSE_OVERLAY}"; then
            fail "secret pattern '${pat}' found in ${COMPOSE_OVERLAY} - remove before shipping"
        fi
    done
    log "secret scan: clean"
else
    log "no UI Compose overlay found - skipping secret scan"
fi

# 5. Smoke: UI /healthz
log "smoke: GET http://localhost:${UI_PORT}/healthz"
curl -sf "http://localhost:${UI_PORT}/healthz" >/dev/null \
    || fail "UI /healthz did not return 200"

# 6. Smoke: UI root
log "smoke: GET http://localhost:${UI_PORT}/"
curl -sf "http://localhost:${UI_PORT}/" >/dev/null \
    || fail "UI / did not return 200"

# 7. Smoke: static asset
log "smoke: GET http://localhost:${UI_PORT}/assets/app.js"
curl -sf "http://localhost:${UI_PORT}/assets/app.js" >/dev/null \
    || fail "UI /assets/app.js did not return 200"

# 8. Smoke: proxied API - accept 404 (backend main), 401 (protected route), or 200 (implemented route).
log "smoke: GET http://localhost:${UI_PORT}/api/system"
API_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${UI_PORT}/api/system")
case "${API_STATUS}" in
    404|401|200) log "  /api/system -> ${API_STATUS} (ok)" ;;
    5??) fail "/api/system returned ${API_STATUS} - backend error" ;;
    *) log "  /api/system -> ${API_STATUS} (unexpected but not blocking)" ;;
esac

# 9. Smoke: bad proxy returns non-200 (proves proxy failures distinguishable from route misses)
log "smoke: GET http://localhost:${BAD_PROXY_PORT}/api/system"
BAD_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${BAD_PROXY_PORT}/api/system")
if [[ "${BAD_STATUS}" == "200" ]]; then
    fail "bad proxy /api/system returned 200 - proxy failure indistinguishable from success"
fi
log "  bad proxy /api/system -> ${BAD_STATUS} (ok - non-200 distinguishes from success)"

log "smoke-ui-docker-e2e: all checks passed"
