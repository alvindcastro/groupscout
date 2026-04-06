package storage

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// schema is applied on every startup. All statements use IF NOT EXISTS so this
// is safe to run repeatedly (idempotent). When we add golang-migrate in Phase 3
// for versioned migrations, this will be replaced by the migrate runner.
const schema = `
CREATE TABLE IF NOT EXISTS raw_projects (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    external_id  TEXT,
    raw_data     TEXT NOT NULL,      -- JSON blob of the original payload
    collected_at DATETIME NOT NULL,
    hash         TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS leads (
    id                        TEXT PRIMARY KEY,
    raw_project_id            TEXT REFERENCES raw_projects(id),
    source                    TEXT,
    title                     TEXT,
    location                  TEXT,
    project_value             INTEGER,
    general_contractor        TEXT,
    project_type              TEXT,
    estimated_crew_size       INTEGER,
    estimated_duration_months INTEGER,
    out_of_town_crew_likely   INTEGER DEFAULT 0,  -- 0=false, 1=true
    priority_score            INTEGER,
    priority_reason           TEXT,
    suggested_outreach_timing TEXT,
    applicant                 TEXT,   -- raw applicant from permit (may include phone)
    contractor                TEXT,   -- raw contractor from permit (may include phone)
    source_url                TEXT,   -- direct link to the source document (PDF, page, etc.)
    notes                     TEXT,
    status                    TEXT DEFAULT 'new', -- new|contacted|proposal|booked|lost
    created_at                DATETIME NOT NULL,
    updated_at                DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS outreach_log (
    id        TEXT PRIMARY KEY,
    lead_id   TEXT REFERENCES leads(id),
    contact   TEXT,
    channel   TEXT,     -- email|linkedin|phone
    notes     TEXT,
    outcome   TEXT,
    logged_at DATETIME NOT NULL
);
`

// Open opens the database at dsn (a file path for SQLite local dev,
// or a postgres:// URL for Postgres).
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open(DriverName(dsn), dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// DriverName returns "pgx" if dsn has a postgres scheme, else "sqlite".
func DriverName(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "pgx"
	}
	return "sqlite"
}

// Migrate applies the schema to the database. Safe to call on every startup.
// For existing databases, it also adds any new columns via ALTER TABLE.
func Migrate(db *sql.DB, dsn string) error {
	if DriverName(dsn) == "pgx" {
		_, b, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(b)
		migrationsPath := filepath.Join(basepath, "..", "..", "migrations")
		m, err := migrate.New("file://"+filepath.ToSlash(migrationsPath), dsn)
		if err != nil {
			return err
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return err
		}
		return nil
	}

	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Idempotent column additions for databases created before these columns existed.
	// SQLite returns "duplicate column name" when a column already exists — we ignore that
	// specific error and surface anything else.
	for _, stmt := range []string{
		`ALTER TABLE leads ADD COLUMN applicant TEXT`,
		`ALTER TABLE leads ADD COLUMN contractor TEXT`,
		`ALTER TABLE leads ADD COLUMN source_url TEXT`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}
