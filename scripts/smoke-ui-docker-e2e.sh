#!/usr/bin/env bash
# Phase 38 backend-owned UI Docker E2E smoke gate.
# Verifies the backend Compose stack + external UI production container.
# The production UI Compose profile lives in the UI repo.
set -euo pipefail

GROUPSCOUT_UI_REPO="${GROUPSCOUT_UI_REPO:-/mnt/c/Users/alvin/WebstormProjects/groupscout-ui}"
BACKEND_PORT="${GROUPSCOUT_BACKEND_HOST_PORT:-8080}"
UI_PORT="${GROUPSCOUT_UI_PRODUCTION_HOST_PORT:-3002}"

log() { echo "[smoke-ui-docker-e2e] $*"; }
fail() { echo "[smoke-ui-docker-e2e] FAIL: $*" >&2; exit 1; }

# Cleanup: bring down UI containers without removing backend data volumes.
cleanup() {
    log "cleanup: stopping UI Compose containers..."
    docker compose \
        --project-directory "${GROUPSCOUT_UI_REPO}" \
        --profile smoke-ui-e2e \
        down --remove-orphans 2>/dev/null || true
    log "cleanup: done"
}
trap cleanup EXIT

# 1. Backend health
log "checking backend health at :${BACKEND_PORT}/health"
curl -sf "http://localhost:${BACKEND_PORT}/health" >/dev/null \
    || fail "backend not healthy at :${BACKEND_PORT}/health — start the backend Compose stack first"

# 2. Validate UI Compose config
log "validating UI Compose profile smoke-ui-e2e"
docker compose \
    --project-directory "${GROUPSCOUT_UI_REPO}" \
    --profile smoke-ui-e2e \
    config --quiet \
    || fail "UI Compose config invalid in ${GROUPSCOUT_UI_REPO}"

# 3. Start UI production containers
log "starting UI production containers (profile smoke-ui-e2e)"
docker compose \
    --project-directory "${GROUPSCOUT_UI_REPO}" \
    --profile smoke-ui-e2e \
    up -d --wait

# 4. Secret scan on UI Compose overlay
PATTERNS=("API_TOKEN" "DATABASE_URL" "SLACK_WEBHOOK_URL" "RESEND_API_KEY" "CLAUDE_API_KEY" "OLLAMA_ENDPOINT" "UI_SESSION_SECRET")
COMPOSE_OVERLAY=""
for candidate in \
    "${GROUPSCOUT_UI_REPO}/docker-compose.smoke-ui-e2e.yml" \
    "${GROUPSCOUT_UI_REPO}/compose.smoke-ui-e2e.yml" \
    "${GROUPSCOUT_UI_REPO}/docker-compose.yml"; do
    if [[ -f "${candidate}" ]]; then
        COMPOSE_OVERLAY="${candidate}"
        break
    fi
done

if [[ -n "${COMPOSE_OVERLAY}" ]]; then
    log "scanning ${COMPOSE_OVERLAY} for leaked secrets"
    for pat in "${PATTERNS[@]}"; do
        if grep -q "${pat}" "${COMPOSE_OVERLAY}"; then
            fail "secret pattern '${pat}' found in ${COMPOSE_OVERLAY} — remove before shipping"
        fi
    done
    log "secret scan: clean"
else
    log "no UI Compose overlay found — skipping secret scan"
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

# 8. Smoke: proxied API — accept 404 (backend main) or 401 (protected route)
log "smoke: GET http://localhost:${UI_PORT}/api/system"
API_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${UI_PORT}/api/system")
case "${API_STATUS}" in
    404|401|200) log "  /api/system → ${API_STATUS} (ok)" ;;
    5??) fail "/api/system returned ${API_STATUS} — backend error" ;;
    *) log "  /api/system → ${API_STATUS} (unexpected but not blocking)" ;;
esac

# 9. Smoke: bad proxy returns non-200 (proves proxy failures distinguishable from route misses)
log "smoke: GET http://localhost:${UI_PORT}/api/bad-proxy-test"
BAD_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${UI_PORT}/api/bad-proxy-test")
if [[ "${BAD_STATUS}" == "200" ]]; then
    fail "/api/bad-proxy-test returned 200 — proxy failure indistinguishable from success"
fi
log "  /api/bad-proxy-test → ${BAD_STATUS} (ok — non-200 distinguishes from success)"

log "smoke-ui-docker-e2e: all checks passed"
