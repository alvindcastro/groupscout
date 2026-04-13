//go:build integration

package enrichment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := storage.Migrate(db, dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM leads")
		db.Exec("DELETE FROM raw_projects")
		db.Exec("DELETE FROM raw_inputs")
		db.Close()
	})
	return db, dsn
}

type mockAIForIntegration struct {
	enrich func(p collector.RawProject) (*EnrichedLead, error)
}

func (m *mockAIForIntegration) Enrich(ctx context.Context, p collector.RawProject) (*EnrichedLead, error) {
	return m.enrich(p)
}

func (m *mockAIForIntegration) DraftOutreach(ctx context.Context, l storage.Lead) (string, error) {
	return "mock outreach", nil
}

func TestEnricher_RawInputDeduplication(t *testing.T) {
	db, dsn := newTestDB(t)
	ctx := context.Background()

	rawStore := storage.NewRawProjectStoreWithDSN(db, dsn)
	auditStore := storage.NewAuditStoreWithDSN(db, dsn)
	leadStore := storage.NewLeadStoreWithDSN(db, dsn)

	ai := &mockAIForIntegration{
		enrich: func(p collector.RawProject) (*EnrichedLead, error) {
			return &EnrichedLead{PriorityScore: 5}, nil
		},
	}

	e := NewEnricher(nil, rawStore, auditStore, leadStore, ai, NewScorer(0), 0, nil, nil, false, false)

	commonRawData := []byte("<rss><item>Project A</item><item>Project B</item></rss>")

	p1 := collector.RawProject{
		Source:     "test-rss",
		ExternalID: "A",
		Title:      "Project A",
		RawData:    commonRawData,
		RawType:    "application/xml",
		Hash:       "hash-A", // Simulated item hash
	}

	p2 := collector.RawProject{
		Source:     "test-rss",
		ExternalID: "B",
		Title:      "Project B",
		RawData:    commonRawData,
		RawType:    "application/xml",
		Hash:       "hash-B", // Simulated item hash
	}

	// Process first project
	inserted1, err := e.processProject(ctx, p1)
	require.NoError(t, err)
	assert.True(t, inserted1)

	// Process second project (same raw source)
	inserted2, err := e.processProject(ctx, p2)
	require.NoError(t, err)
	assert.True(t, inserted2)

	// Verify leads
	leads, err := leadStore.ListNew(ctx)
	require.NoError(t, err)
	require.Len(t, leads, 2)

	assert.NotEmpty(t, leads[0].RawInputID)
	assert.NotEmpty(t, leads[1].RawInputID)

	// CRITICAL: They should share the same RawInputID
	assert.Equal(t, leads[0].RawInputID, leads[1].RawInputID, "Leads from same raw data should share RawInputID")

	// Verify only one RawInput record exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM raw_inputs").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Only one RawInput record should exist for shared raw data")

	// Verify the hash stored in raw_inputs is the payload hash, not the item hash
	var storedHash string
	err = db.QueryRow("SELECT hash FROM raw_inputs").Scan(&storedHash)
	require.NoError(t, err)

	expectedPayloadHash := fmt.Sprintf("%x", sha256.Sum256(commonRawData))
	assert.Equal(t, expectedPayloadHash, storedHash, "Stored hash should be the payload hash")
}
