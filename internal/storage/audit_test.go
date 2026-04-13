package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSQLiteDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open SQLite: %v", err)
	}
	if err := Migrate(db, ":memory:"); err != nil {
		t.Fatalf("Migrate SQLite: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db, ":memory:"
}

func TestAuditStore_PurgeOlderThan(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewAuditStoreWithDSN(db, dsn)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second) // SQLite might truncate or handle milliseconds differently
	oldTime := now.Add(-24 * time.Hour)
	veryOldTime := now.Add(-48 * time.Hour)

	// 1. Insert an old record
	_, err := store.Store(ctx, RawInput{
		Hash:      "old-hash",
		Payload:   []byte("old"),
		CreatedAt: veryOldTime,
	})
	require.NoError(t, err)

	// 2. Insert a new record
	_, err = store.Store(ctx, RawInput{
		Hash:      "new-hash",
		Payload:   []byte("new"),
		CreatedAt: now,
	})
	require.NoError(t, err)

	// 3. Purge older than oldTime (should delete the very old one)
	count, err := store.PurgeOlderThan(ctx, oldTime)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 4. Verify only new one remains
	exists, err := store.ExistsByHash(ctx, "old-hash")
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = store.ExistsByHash(ctx, "new-hash")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestAuditStore_PurgeOlderThan_Referenced(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewAuditStoreWithDSN(db, dsn)
	leadStore := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	oldTime := now.Add(-24 * time.Hour)
	veryOldTime := now.Add(-48 * time.Hour)

	// 1. Insert an old record
	id, err := store.Store(ctx, RawInput{
		Hash:      "old-referenced-hash",
		Payload:   []byte("old"),
		CreatedAt: veryOldTime,
	})
	require.NoError(t, err)

	// 2. Reference it from a lead
	err = leadStore.Insert(ctx, &Lead{
		ID:         "test-lead-id",
		Source:     "test",
		Title:      "Test Lead",
		RawInputID: id.String(),
		Status:     "new",
	})
	require.NoError(t, err)

	// 3. Purge older than oldTime (should NOT delete the referenced one)
	count, err := store.PurgeOlderThan(ctx, oldTime)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 4. Verify it still exists
	exists, err := store.ExistsByHash(ctx, "old-referenced-hash")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestAuditStore_StripPII(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	// Create store with piiStrip enabled
	store := &sqlAuditStore{db: db, dsn: dsn, piiStrip: true}
	ctx := context.Background()

	payload := []byte("Contact alvin@example.com or 604-555-0199 for more info.")
	hash := "pii-hash"

	_, err := store.Store(ctx, RawInput{
		Hash:    hash,
		Payload: payload,
	})
	require.NoError(t, err)

	got, err := store.GetByHash(ctx, hash)
	require.NoError(t, err)

	// Check if PII was stripped
	s := string(got.Payload)
	assert.NotContains(t, s, "alvin@example.com")
	assert.NotContains(t, s, "604-555-0199")
	assert.Contains(t, s, "[REDACTED EMAIL]")
	assert.Contains(t, s, "[REDACTED PHONE]")
}
