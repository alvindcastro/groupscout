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

### 7. Future Testing Goals
- **Mocking External APIs**: Implementing more robust mocking for Slack and Claude APIs to reduce dependency on network calls during CI.
- **Load Testing**: Verifying the performance of the collector registry and worker pools under high concurrency (Phase 9).
