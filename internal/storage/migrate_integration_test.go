//go:build integration

package storage

import (
	"os"
	"testing"
)

func TestMigrate_postgres_tables_exist(t *testing.T) {
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

	tables := []string{"raw_projects", "leads", "outreach_log", "raw_inputs"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM information_schema.tables
             WHERE table_schema='public' AND table_name=$1`, table,
		).Scan(&count)
		if err != nil || count == 0 {
			t.Errorf("table %q does not exist after migration", table)
		}
	}
}

func TestMigrate_postgres_column_types(t *testing.T) {
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

	tests := []struct {
		table, column, wantType string
	}{
		{"leads", "id", "uuid"},
		{"leads", "raw_input_id", "uuid"},
		{"leads", "out_of_town_crew_likely", "boolean"},
		{"leads", "created_at", "timestamp with time zone"},
		{"leads", "project_value", "bigint"},
		{"raw_projects", "raw_data", "jsonb"},
		{"raw_inputs", "payload", "bytea"},
	}
	for _, tt := range tests {
		var dataType string
		err := db.QueryRow(`
            SELECT data_type FROM information_schema.columns
            WHERE table_name=$1 AND column_name=$2`, tt.table, tt.column,
		).Scan(&dataType)
		if err != nil {
			t.Errorf("%s.%s: query error: %v", tt.table, tt.column, err)
			continue
		}
		if dataType != tt.wantType {
			t.Errorf("%s.%s type = %q, want %q", tt.table, tt.column, dataType, tt.wantType)
		}
	}
}

func TestMigrate_idempotent(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	// Running twice must not error
	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestMigrate_sqlite_still_works(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()
	if err := Migrate(db, ":memory:"); err != nil {
		t.Fatalf("SQLite Migrate: %v", err)
	}
}
