#!/usr/bin/env bash
set -euo pipefail

BACKEND_REPO="${GROUPSCOUT_BACKEND_REPO:-/mnt/c/Users/alvin/GolandProjects/groupscout}"
UI_REPO="${GROUPSCOUT_UI_REPO:-/mnt/c/Users/alvin/WebstormProjects/groupscout-ui}"
PROJECT="${GROUPSCOUT_DOCKER_PROJECT:-groupscout}"
UI_PORT="${GROUPSCOUT_UI_PRODUCTION_HOST_PORT:-3002}"
BAD_PROXY_PORT="${GROUPSCOUT_UI_BAD_PROXY_HOST_PORT:-3003}"

BACKEND_COMPOSE="${BACKEND_REPO}/docker-compose.yml"
UI_COMPOSE="${UI_REPO}/compose.dev.yml"

compose() {
  docker compose \
    -p "${PROJECT}" \
    -f "${BACKEND_COMPOSE}" \
    -f "${UI_COMPOSE}" \
    --profile smoke-ui-e2e \
    "$@"
}

cleanup() {
  compose stop groupscout-ui-production groupscout-ui-production-bad-proxy >/dev/null 2>&1 || true
  compose rm -f groupscout-ui-production groupscout-ui-production-bad-proxy >/dev/null 2>&1 || true
}

wait_for_http() {
  local url="$1"
  local expected="$2"
  local label="$3"

  for _ in $(seq 1 60); do
    local status
    status="$(curl --max-time 5 -sS -o /tmp/groupscout-ui-smoke-response.txt -w '%{http_code}' "${url}" || true)"
    if [[ "${status}" =~ ^(${expected})$ ]]; then
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for ${label} at ${url}; last status=${status:-none}" >&2
  if [[ -f /tmp/groupscout-ui-smoke-response.txt ]]; then
    cat /tmp/groupscout-ui-smoke-response.txt >&2
  fi
  return 1
}

assert_body_contains() {
  local url="$1"
  local needle="$2"
  local label="$3"

  local body
  body="$(curl -fsS "${url}")"
  if [[ "${body}" != *"${needle}"* ]]; then
    echo "Expected ${label} response from ${url} to contain ${needle}" >&2
    return 1
  fi
}

assert_compose_has_no_browser_secrets() {
  if grep -Eiq 'API_TOKEN|DATABASE_URL|POSTGRES_URL|SLACK|RESEND|SENDGRID|OPENAI|ANTHROPIC|CLAUDE|OLLAMA|UI_SESSION_SECRET' "${UI_COMPOSE}"; then
    echo "UI Compose overlay contains a forbidden browser/runtime secret name" >&2
    return 1
  fi
}

trap cleanup EXIT

assert_compose_has_no_browser_secrets
compose config --quiet
compose up -d --build groupscout groupscout-ui-production groupscout-ui-production-bad-proxy

wait_for_http "http://localhost:8080/health" "200" "backend health"
wait_for_http "http://localhost:${UI_PORT}/healthz" "200" "production UI health"
wait_for_http "http://localhost:${UI_PORT}/" "200" "production UI root"
wait_for_http "http://localhost:${UI_PORT}/assets/app.js" "200" "production UI static asset"
wait_for_http "http://localhost:${UI_PORT}/api/system" "401|404" "production UI API proxy reachability"
wait_for_http "http://localhost:${BAD_PROXY_PORT}/healthz" "200" "bad-proxy UI health"
wait_for_http "http://localhost:${BAD_PROXY_PORT}/api/system" "502" "bad-proxy UI upstream failure"

assert_body_contains "http://localhost:${UI_PORT}/" "GroupScout" "production UI root"
assert_body_contains "http://localhost:${UI_PORT}/assets/app.js" "/api/system" "production UI static asset"

cleanup
echo "groupscout-ui-docker-e2e-ok"
