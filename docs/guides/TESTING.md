### GroupScout Testing Strategy

The GroupScout testing infrastructure ensures the reliability of the lead collection, enrichment, and notification pipeline. It focuses on unit testing, data parsing verification, and end-to-end integration checks.

### 1. Automated Go Tests (Makefile)

The project includes standard Go unit and integration tests. The recommended way to run them is via the `Makefile`.

#### Run All Tests
```bash
make test
```

#### Run Specific Package Tests
```bash
go test -v ./internal/enrichment/...
```

#### Run Alertd Tests
```bash
go test ./cmd/alertd/... ./internal/alert/...
```

---

### 2. Ollama Integration Testing

Since GroupScout relies on local LLMs for extraction and scoring, you need to verify that Ollama is correctly configured and the necessary models are available.

#### Using the Test Script
We provide a helper script to check Ollama connectivity, model availability, and basic inference:
```bash
./scripts/test-ollama.sh
```

#### Manual Verification
1.  **Check if Ollama is running**: `curl http://localhost:11434/api/tags`
2.  **Pull required base models**:
    ```bash
    make ollama-pull
    # OR
    ollama pull mistral
    ollama pull llama3.1:8b
    ollama pull phi3:mini
    ```
3.  **Push persona Modelfiles**:
    This creates the specific personas (like `permit_extractor`) used by GroupScout:
    ```bash
    make ollama-push
    # OR
    go run cmd/server/main.go ollama push-models
    ```

---

### 3. Manual Verification & Tools
Several utility scripts are provided for manual verification:
- `scripts/test-ollama.sh`: Verifies Ollama connectivity and models.
- `cmd/test_sentry/main.go`: Verifies Sentry connectivity and error reporting.
- `check_db.go`: A quick script to inspect the contents of the SQLite `groupscout.db`.
- `/run` endpoint: Allows triggering a full pipeline execution manually via HTTP.

### 4. Integration Testing
Integration tests are available for the storage layer and require a running database instance.

- **SQLite**: Standard tests run on SQLite by default.
- **Postgres**: Integration tests for Postgres are gated by the `TEST_POSTGRES_URL` environment variable and the `integration` build tag.

**Run Postgres integration tests:**
```powershell
$env:TEST_POSTGRES_URL="postgres://groupscout:groupscout@localhost:5432/groupscout?sslmode=disable"
go test -v -tags integration ./internal/storage/...
```

These tests verify:
- Dynamic driver selection (`DriverName`).
- SQL placeholder rebinding (`Rebind`).
- Versioned migrations (`Migrate`) using `golang-migrate`.
- Native Postgres type handling (e.g., `BOOLEAN`, `JSONB`).
- Vector similarity search (`EmbeddingStore`) using `pgvector` operators (e.g., `<=>`).
- CRUD operations and idempotency.

**Run embedding-specific unit tests (SQLite/Go-native):**
```powershell
go test -v ./internal/storage/ -run EmbeddingStore
```

**Trigger the pipeline manually (Docker):**
```bash
curl -X POST http://localhost:8080/run \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

**Check what happened after a run:**
```bash
docker compose logs app --tail=50
```

**Follow logs in real time during a run:**
```bash
docker compose logs -f app
```

### 5. Collector Test Pattern
When adding a new collector, follow the pattern used in `internal/collector/richmond_test.go`:
1. Define a `sampleLines` or `sampleHTML` variable with representative raw data.
2. Write tests for individual parsing helper functions (e.g., `parseDate`, `parseValue`).
3. Write a high-level test for the `Collect` or `process` function using a mock implementation of the source if possible.

### 6. CI/CD & Reliability
- **Deduplication**: Tests in `leads_test.go` (if implemented) or during integration ensure that the same lead is not processed multiple times.
- **Error Handling**: The Sentry integration (Phase 8.2) captures runtime exceptions, ensuring that transient failures in collectors are visible in the observability dashboard.

### 7. API Testing

For detailed instructions on testing API endpoints via `curl`, Bruno, or Swagger, see the [API Testing section](#8-api-testing-details) below.

### 8. API Testing Details

#### 8.1 Prerequisite: API Token
Most POST endpoints require a Bearer Token for authentication. This token is defined by the `API_TOKEN` environment variable in your `.env` file. If `API_TOKEN` is not set, the server will allow all requests (insecure mode, intended for local development only).

#### 8.2 Testing with `curl`

**Health Check (Port 8080)**
```bash
curl -i http://localhost:8080/health
```

**Manual Pipeline Run**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  -H "Content-Type: application/json" \
  -d '{"bcbid_raw_input": ""}' \
  http://localhost:8080/run
```

**Trigger Weekly Digest**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  "http://localhost:8080/digest?to=alvin@groupscout.ai"
```

**n8n Webhook Simulation**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  -H "Content-Type: application/json" \
  -d '{
    "Source": "curl_test",
    "Title": "Simulated Lead",
    "Location": "Vancouver, BC",
    "ProjectValue": 500000
  }' \
  http://localhost:8080/n8n/webhook
```

**Slack Inventory Update (Port 8081)**
```bash
curl -i -X POST \
  -d "text=50" \
  http://localhost:8081/slack/inventory
```

#### 8.3 Testing with Bruno (Recommended)
[Bruno](https://www.usebruno.com/) is a fast, open-source API client. A collection for GroupScout is included in the repository.

1. **Open Bruno**.
2. **Open Collection**: Point it to the `api/bruno` folder in this repository.
3. **Select Environment**: Click the top-right environment selector and choose **Local**.
4. **Configure Token**: Edit the **Local** environment variables to match your `API_TOKEN`.
5. **Run Requests**: You can now run all pre-configured requests.

#### 8.4 Testing with OpenAPI / Swagger
An OpenAPI 3.0 specification is available at `api/swagger.yaml`.
- **Visualizing**: You can paste the content of `api/swagger.yaml` into the [Swagger Editor](https://editor.swagger.io/) or use a local Swagger UI instance.
- **Servers**: The spec defines two servers:
    - `http://localhost:8080` (Lead Generation)
    - `http://localhost:8081` (Alerting Service)

### 10. Audit Trail & Retention Testing

The audit trail includes privacy and retention features that should be verified regularly.

#### Test PII Redaction
Verify that emails and phone numbers are redacted when `PII_STRIP=true` is enabled:
```bash
# This test specifically covers PII stripping logic
go test -v ./internal/storage -run TestAuditStore_StripPII
```

#### Test Retention Purge
Verify that old records are deleted, but referenced ones are kept:
```bash
# This test verifies that PurgeOlderThan ignores referenced records
go test -v ./internal/storage -run TestAuditStore_PurgeOlderThan
```

#### Manual Verification via SQL
You can verify the state of your live database (Postgres) using the queries provided in [POSTGRES_QUERIES.md](./POSTGRES_QUERIES.md#audit-trail-raw-inputs).

---

### 11. LUX MVP Testing (n8n only)

LUX MVP workflows (MVP-A, MVP-B, MVP-C) run entirely inside n8n and call external APIs (Anthropic, Airtable, Slack). They do **not** require the GroupScout Go server, Postgres, or Ollama. Only n8n needs to be running.

#### Start n8n in Isolation

```bash
# Start only n8n — no other services needed
docker compose up -d n8n

# Confirm running
docker compose ps n8n

# Tail logs during test runs
docker compose logs -f n8n
```

n8n UI: `http://localhost:5678`

```bash
# Stop when done
docker compose stop n8n
```

---

#### MVP-B — Lead Follow-Up Sequence

**Prerequisites:** Anthropic API key, Airtable base with `Leads` table, Slack bot in `#new-leads`.
See [LUX_MVP_B_SETUP.md](LUX_MVP_B_SETUP.md) for credential setup.

**Test 1 — Commercial lead (high tier, routes to Prompt 3A)**

```bash
curl -X POST http://localhost:5678/webhook/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-b/payload.json
```

Expected response: `{"status":"queued","lead":"Marcus Webb","tier":"high"}`

Verify:
- [ ] Airtable: record created with `Lead Tier: high`, `Project Type: commercial_renovation`
- [ ] Email 1 body opens with "tenant spaces" (mirrors Marcus's exact words)
- [ ] Email 2 contains a commercial-specific insight (phased permitting, tenant coordination) — not design-build
- [ ] Slack `#new-leads`: flags high-tier, ends with ownership question

**Test 2 — Residential lead (medium tier, routes to Prompt 3B)**

```bash
curl -X POST http://localhost:5678/webhook/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-b/payload_alt.json
```

Expected response: `{"status":"queued","lead":"Jennifer Okafor","tier":"medium"}`

Verify:
- [ ] Airtable: record created with `Lead Tier: medium`, `Project Type: custom_home`
- [ ] Email 2 contains a residential-specific insight (design-build, scope creep) — not phased permitting
- [ ] Email 3 is noticeably softer than the commercial version
- [ ] `Urgency Signal` checkbox is unchecked ("maybe next year" is not an urgency signal)

**Test 3 — Routing check**

In **Executions** → latest run → open **Route by Category** node:
- `payload.json` (`commercial_renovation`) → True branch → **Generate Commercial Sequence**
- `payload_alt.json` (`custom_home`) → False branch → **Generate Residential Sequence**

---

#### MVP-A — Client Status Email

```bash
curl -X POST http://localhost:5678/webhook/lux-status-email \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-a/payload.json
```

Verify: Slack `#client-updates-review` receives a Block Kit message with Approve/Edit buttons.

```bash
# Alt payload — budget overrun + supplier delay
curl -X POST http://localhost:5678/webhook/lux-status-email \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-a/payload_alt.json
```

Verify: email body addresses the budget situation plainly, does not use the word "variance".

---

#### MVP-C — LinkedIn Post Pipeline

```bash
curl -X POST http://localhost:5678/webhook/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_milestone.json

curl -X POST http://localhost:5678/webhook/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_podcast.json
```

Verify: Slack `#content-review` receives the post draft for each payload.

---

#### Using n8n Test Mode (no activation needed)

For iterative prompt testing without activating the workflow:

1. Open the workflow in the n8n editor
2. Click **Test workflow** (top-right) — n8n listens on the test URL
3. Send a payload using `/webhook-test/` instead of `/webhook/`:

```bash
curl -X POST http://localhost:5678/webhook-test/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-b/payload.json
```

Execution results appear inline, node by node. The test URL only responds while the editor is open in test mode.

---

#### Checking Executions

Go to **Executions** in the left sidebar. Click any node in a past run to see its exact input and output — this is the fastest way to debug classification drift, routing errors, or JSON parse failures.

See [LUX_MVP_B_TROUBLESHOOTING.md](LUX_MVP_B_TROUBLESHOOTING.md) for node-by-node failure analysis.
