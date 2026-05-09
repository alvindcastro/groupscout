# Phase 38 Docker Runtime And End-To-End Smoke Contract

Phase 38 adds a backend-owned smoke script for the split backend plus external UI Docker runtime.

Script:

- `scripts/smoke-ui-docker-e2e.sh`

The script assumes the UI repo lives at `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui` unless `GROUPSCOUT_UI_REPO` overrides it.

## Checks

The smoke script verifies:

- Backend Compose health: `GET http://localhost:8080/health`.
- UI D3 health harness: `GET /healthz` on `${GROUPSCOUT_UI_HOST_PORT:-3001}`.
- Production UI container serves `/healthz`, `/`, and `/assets/app.js` on `${GROUPSCOUT_UI_PROD_PORT:-3002}`.
- Same-origin proxy reaches backend `/api/leads?limit=1`.
- Same-origin proxy reaches backend `/api/system`.
- Same-origin proxy reaches backend `/api/alerts?limit=1`.
- A deliberately bad proxy target returns `502`, proving proxy failures are distinguishable from backend route misses.
- UI model-level route smoke runs `node --test test/app-shell.test.js test/lead-inbox-screen.test.js test/lead-detail-screen.test.js`.
- Static assets do not contain `API_TOKEN`, database URLs, Slack/email/LLM keys, session secrets, bearer tokens, private key markers, or `http://groupscout:8080` in browser assets.

## Evidence

Red evidence:

- `go test ./internal/smoke` failed because `scripts/smoke-ui-docker-e2e.sh` did not exist.

Green evidence:

- `go test ./internal/smoke`

Runtime Docker smoke was not executed during this implementation pass; the script is the executable runbook for a host with Docker and the external UI repo available.
