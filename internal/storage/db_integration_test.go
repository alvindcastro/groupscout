//go:build integration

package storage

import (
	"os"
	"testing"
)

func TestOpen_postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open(%q) error: %v", dsn, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestOpen_sqlite_still_works(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(':memory:') error: %v", err)
	}
	defer db.Close()
}
