package storage

import (
	"context"
	"path/filepath"
	"testing"
)

// The SQLite dev database gets new lead columns from Migrate's ALTER TABLE
// list, including databases created before the columns existed.
func TestLeadStore_SQLiteAIProviderColumns(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "groupscout.db")
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Simulate a database from before the provider chain, then migrate again.
	for _, stmt := range []string{`ALTER TABLE leads DROP COLUMN ai_provider`, `ALTER TABLE leads DROP COLUMN ai_model`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("Migrate on an old database: %v", err)
	}

	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()
	lead := &Lead{Source: "test", Title: "scored", Status: "new", PriorityScore: 7, AIProvider: "claude", AIModel: "claude-haiku-4-5-20251001"}
	if err := store.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	got, err := store.GetByID(ctx, lead.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v, %v", got, err)
	}
	if got.AIProvider != "claude" || got.AIModel != "claude-haiku-4-5-20251001" {
		t.Errorf("AIProvider/AIModel = %q/%q", got.AIProvider, got.AIModel)
	}
}
