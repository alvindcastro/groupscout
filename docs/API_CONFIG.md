### API & Endpoint Configuration Guide

This document lists all internal and external API endpoints used by GroupScout and specifies which are configurable via environment variables.

---

### 1. Internal API Endpoints (Inbound)

These endpoints are exposed by the GroupScout binaries for external triggers (e.g., n8n, Slack, or monitoring).

#### Lead Generation Server (`cmd/server`)
Default port: `8080` (Configurable via `PORT`)

| Endpoint | Method | Description | Auth Required |
|---|---|---|---|
| `/health` | `GET` | System health check (DB + API connectivity). | No |
| `/run` | `POST` | Manually triggers the collect-enrich-notify pipeline. | Yes (Bearer Token) |
| `/digest` | `POST` | Sends a weekly email summary of leads via Resend. | Yes (Bearer Token) |
| `/n8n/webhook` | `POST` | Receives raw lead data from n8n for storage and enrichment. | Yes (Bearer Token) |

#### Alertd Binary (`cmd/alertd`)
Default port: `8081` (Configurable via `ALERTD_PORT`)

| Endpoint | Method | Description | Auth Required |
|---|---|---|---|
| `/slack/inventory` | `POST` | Slack slash command to update real-time room availability. | No (Slack verification recommended) |

---

### 2. External Service Integrations (Outbound)

GroupScout integrates with several external SaaS providers.

| Service | Environment Variable | Default / Base URL | Purpose |
|---|---|---|---|
| **Anthropic** | `CLAUDE_API_KEY` | `https://api.anthropic.com/v1/messages` | AI lead enrichment and outreach drafting. |
| **Google Gemini** | `GEMINI_API_KEY` | `https://generativelanguage.googleapis.com/...` | Alternative AI provider. |
| **Slack Webhooks** | `SLACK_WEBHOOK_URL` | `https://hooks.slack.com/services/...` | Posting lead digests to channels. |
| **Slack Bot API** | `SLACK_BOT_TOKEN` | `https://slack.com/api/...` | Used by `alertd` for stateful alert updates. |
| **Resend** | `RESEND_API_KEY` | `https://api.resend.com/emails` | Sending weekly lead summary emails. |
| **Hunter.io** | `HUNTER_API_KEY` | `https://api.hunter.io/v2/...` | (Phase 18) Finding decision-maker contact info. |
| **Sentry** | `SENTRY_DSN` | - | Error tracking and observability. |

---

### 3. Data Source Polling (Scrapers)

The following URLs are polled by collectors to gather raw data.

| Source | Environment Variable | Current Default URL | Status |
|---|---|---|---|
| **Richmond Permits** | `RICHMOND_PERMITS_URL` | `https://www.richmond.ca/.../*.pdf` | Configurable |
| **Delta Permits** | `DELTA_PERMITS_URL` | `https://delta.ca/building-permits` | Configurable |
| **CreativeBC** | `CREATIVEBC_URL` | `https://www.creativebc.com/...` | Configurable |
| **VCC Events** | `VCC_URL` | `https://www.vancouverconventioncentre.com/...` | Configurable |
| **BC Bid RSS** | `BCBID_RSS_URL` | `https://www.civicinfo.bc.ca/rss/...` | Configurable |
| **News RSS** | `NEWS_RSS_URL` | `https://news.google.com/rss/...` | Configurable |
| **Eventbrite** | `EVENTBRITE_URL` | `https://www.eventbrite.ca/...` | Configurable |
| **ECCC Weather** | `ECCC_ALERTS_URL` | `https://api.weather.gc.ca/collections/alerts/items` | Configurable |
| **YVR Flight Status**| `YVR_FLIGHT_STATUS_URL` | `https://www.yvr.ca/en/passengers/flights` | Configurable |
| **NavCanada NOTAM** | `NAVCANADA_NOTAM_URL` | `https://www.navcanada.ca/.../notam-portal.aspx` | Configurable |

---

### 4. Implementation Notes

-   **Internal APIs**: The Lead Generation Server and Alertd are separate binaries with distinct ports.
-   **Security**: Use the `API_TOKEN` (Bearer Auth) for all sensitive POST endpoints.
-   **Extensibility**: Adding new data sources usually requires a new entry in the Scrapers table and a corresponding URL in the `.env` file.
-   **Testing**: See [API_TESTING.md](./API_TESTING.md) for a guide on how to test these endpoints.
