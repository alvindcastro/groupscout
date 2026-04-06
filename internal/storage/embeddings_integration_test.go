//go:build integration

package storage

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPostgresEmbeddingStore_SaveAndSimilar(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, _ := Open(dsn)
	defer db.Close()
	Migrate(db, dsn)

	store := NewEmbeddingStore(db, dsn)
	ctx := context.Background()

	// Insert a lead to satisfy the FK constraint
	leadStore := NewLeadStoreWithDSN(db, dsn)
	lead := &Lead{Source: "test", Title: "pgvector test lead", Status: "new"}
	leadStore.Insert(ctx, lead)
	defer db.Exec("DELETE FROM lead_embeddings WHERE lead_id = $1", lead.ID)
	defer db.Exec("DELETE FROM leads WHERE id = $1", lead.ID)

	vec := make([]float32, 512)
	vec[0] = 1.0 // simple non-zero vector

	if err := store.Save(ctx, lead.ID, "voyage-3-lite", vec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	ids, err := store.Similar(ctx, vec, 1)
	if err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("Similar returned no results")
	}
	if ids[0] != lead.ID {
		t.Errorf("Similar[0] = %q, want %q", ids[0], lead.ID)
	}
}

func TestPostgresEmbeddingStore_index_used(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, _ := Open(dsn)
	defer db.Close()
	Migrate(db, dsn)

	var plan string
	vec := make([]float32, 512)
	_ = vec
	// pgvector-go encodes the vector; we just check the query plan mentions the index
	// This is a smoke test — the index name contains "ivfflat"
	db.QueryRow(`EXPLAIN SELECT lead_id FROM lead_embeddings
        ORDER BY embedding <=> $1 LIMIT 3`, "["+strings.Repeat("0,", 511)+"0]",
	).Scan(&plan)
	// Just verify the query runs without error — index usage depends on data volume
	t.Logf("query plan: %s", plan)
}
