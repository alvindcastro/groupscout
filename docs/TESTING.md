### GroupScout Testing Strategy

The GroupScout testing infrastructure ensures the reliability of the lead collection, enrichment, and notification pipeline. It focuses on unit testing, data parsing verification, and end-to-end integration checks.

#### 1. Unit Testing
- **Collectors**: Each collector (e.g., `richmond`, `bcbid`, `news`) has a corresponding `_test.go` file. These tests use sample data (representative PDF text, RSS XML, or HTML) to verify parsing logic without making real network calls.
- **Enrichment**: The logic for scoring leads (`scorer_test.go`) and the prompt-based extraction (`claude_test.go`) are tested using mock API responses or predefined test cases.
- **Utility Functions**: Common logic such as dollar amount parsing, date extraction, and deduplication are covered by unit tests.

#### 2. Running Tests
Execute all project tests using:
```powershell
go test ./...
```

To run tests for a specific package:
```powershell
go test ./internal/collector/...
```

#### 3. Manual Verification & Tools
Several utility scripts are provided for manual verification:
- `cmd/test_sentry/main.go`: Verifies Sentry connectivity and error reporting.
- `check_db.go`: A quick script to inspect the contents of the SQLite `groupscout.db`.
- `/run` endpoint: Allows triggering a full pipeline execution manually via HTTP.

#### 4. Integration Testing
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

#### 5. Collector Test Pattern
When adding a new collector, follow the pattern used in `internal/collector/richmond_test.go`:
1. Define a `sampleLines` or `sampleHTML` variable with representative raw data.
2. Write tests for individual parsing helper functions (e.g., `parseDate`, `parseValue`).
3. Write a high-level test for the `Collect` or `process` function using a mock implementation of the source if possible.

#### 6. CI/CD & Reliability
- **Deduplication**: Tests in `leads_test.go` (if implemented) or during integration ensure that the same lead is not processed multiple times.
- **Error Handling**: The Sentry integration (Phase 8.2) captures runtime exceptions, ensuring that transient failures in collectors are visible in the observability dashboard.

#### 7. Future Testing Goals
- **Mocking External APIs**: Implementing more robust mocking for Slack and Claude APIs to reduce dependency on network calls during CI.
- **Load Testing**: Verifying the performance of the collector registry and worker pools under high concurrency (Phase 9).
