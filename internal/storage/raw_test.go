package storage

import (
	"context"
	"os"
	"testing"
	"unicode/utf8"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeText_ValidUTF8(t *testing.T) {
	cases := []string{
		"",
		"richmond_permits",
		"25 036523 000 00 B7",
		"application/pdf",
		"Hôtel • Congrès",
	}
	for _, s := range cases {
		got := sanitizeText(s)
		assert.Equal(t, s, got, "valid UTF-8 string must pass through unchanged")
	}
}

func TestSanitizeText_InvalidUTF8(t *testing.T) {
	// Raw bytes that are not valid UTF-8 (e.g. Latin-1 from a PDF font encoding).
	invalid := string([]byte{0x41, 0x80, 0x42}) // "A\x80B"
	assert.False(t, utf8.ValidString(invalid))

	got := sanitizeText(invalid)
	assert.True(t, utf8.ValidString(got), "result must be valid UTF-8")
	assert.Contains(t, got, "A")
	assert.Contains(t, got, "B")
}

func TestSanitizeText_PreservesASCII(t *testing.T) {
	s := "bcbid|12345|https://example.com"
	assert.Equal(t, s, sanitizeText(s))
}

func TestRawProjectStore_PostgresStoresNonJSONBytes(t *testing.T) {
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
	t.Cleanup(func() {
		db.Exec("DELETE FROM raw_projects WHERE hash = 'raw-bytes-test-hash'")
	})

	store := NewRawProjectStoreWithDSN(db, dsn)
	err = store.Insert(context.Background(), &collector.RawProject{
		Source:     "announcements",
		ExternalID: "bcib:test",
		RawData:    []byte{0x25, 0x50, 0x44, 0x46, 0xe2, 0xe3, 0xcf},
		RawType:    "application/pdf",
		Hash:       "raw-bytes-test-hash",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	exists, err := store.ExistsByHash(context.Background(), "raw-bytes-test-hash")
	if err != nil {
		t.Fatalf("ExistsByHash: %v", err)
	}
	assert.True(t, exists)
}
