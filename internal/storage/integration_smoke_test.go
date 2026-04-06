//go:build integration

package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
)

// TestFullStack_postgres runs the minimum pipeline operations against Postgres
// to confirm the production config works end to end.
func TestFullStack_postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	ctx := context.Background()
	leadStore := NewLeadStore(db)
	rawStore := NewRawProjectStore(db)
	embStore := NewEmbeddingStore(db, dsn)

	// Raw project
	p := &collector.RawProject{
		Source: "smoke_test", Title: "Smoke test permit",
		ExternalID: "SMOKE-001", IssuedAt: time.Now(),
		RawData: map[string]any{"test": true},
	}
	p.Hash = HashProject(p.Source, p.ExternalID, p.Title, p.IssuedAt)
	if err := rawStore.Insert(ctx, p); err != nil {
		t.Errorf("rawStore.Insert: %v", err)
	}

	// Lead
	lead := &Lead{
		Source: "smoke_test",
		Title:  "Smoke test lead", OutOfTownCrewLikely: true,
		PriorityScore: 8, Status: "new",
	}
	if err := leadStore.Insert(ctx, lead); err != nil {
		t.Errorf("leadStore.Insert: %v", err)
	}

	// Embedding
	vec := make([]float32, 512)
	vec[0] = 1.0
	if err := embStore.Save(ctx, lead.ID, "test", vec); err != nil {
		t.Errorf("embStore.Save: %v", err)
	}

	// Cleanup
	db.Exec("DELETE FROM lead_embeddings WHERE lead_id = $1", lead.ID)
	db.Exec("DELETE FROM leads WHERE source = 'smoke_test'")
	db.Exec("DELETE FROM raw_projects WHERE source = 'smoke_test'")
}
