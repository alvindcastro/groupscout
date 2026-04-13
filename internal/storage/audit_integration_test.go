//go:build integration

package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditStore_RoundTrip(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewAuditStoreWithDSN(db, dsn)
	ctx := context.Background()

	payload := []byte("%PDF-1.4\n%test pdf content")
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))

	raw := RawInput{
		Hash:          hash,
		PayloadType:   "pdf",
		Payload:       payload,
		SourceURL:     "https://example.com/test.pdf",
		CollectorName: "test-collector",
	}

	// 1. Store
	id, err := store.Store(ctx, raw)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	// 2. GetByID
	got, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, hash, got.Hash)
	assert.Equal(t, "pdf", got.PayloadType)
	assert.Equal(t, payload, got.Payload)
	assert.Equal(t, "https://example.com/test.pdf", got.SourceURL)
	assert.Equal(t, "test-collector", got.CollectorName)
	assert.False(t, got.CreatedAt.IsZero())

	// 3. GetByHash
	gotByHash, err := store.GetByHash(ctx, hash)
	require.NoError(t, err)
	require.NotNil(t, gotByHash)
	assert.Equal(t, id, gotByHash.ID)
}

func TestAuditStore_GetNonExistent(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewAuditStoreWithDSN(db, dsn)
	ctx := context.Background()

	got, err := store.GetByID(ctx, uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, got)

	gotByHash, err := store.GetByHash(ctx, "non-existent-hash")
	assert.NoError(t, err)
	assert.Nil(t, gotByHash)
}
