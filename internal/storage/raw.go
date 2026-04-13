package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
)

// RawProjectStore is the interface for persisting raw ingest records.
type RawProjectStore interface {
	Insert(ctx context.Context, p *collector.RawProject) error
	ExistsByHash(ctx context.Context, hash string) (bool, error)
}

type sqliteRawStore struct {
	db  *sql.DB
	dsn string
}

// NewRawProjectStore returns a RawProjectStore.
func NewRawProjectStore(db *sql.DB) RawProjectStore {
	return &sqliteRawStore{db: db}
}

// NewRawProjectStoreWithDSN returns a RawProjectStore that knows its DSN for rebinding.
func NewRawProjectStoreWithDSN(db *sql.DB, dsn string) RawProjectStore {
	return &sqliteRawStore{db: db, dsn: dsn}
}

func (s *sqliteRawStore) Insert(ctx context.Context, p *collector.RawProject) error {
	query := `
		INSERT INTO raw_projects (id, source, external_id, raw_data, raw_type, collected_at, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hash) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		NewUUID(), p.Source, p.ExternalID, string(p.RawData), p.RawType, time.Now().UTC(), p.Hash)
	return err
}

func (s *sqliteRawStore) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	var count int
	query := `SELECT COUNT(1) FROM raw_projects WHERE hash = ?`
	err := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), hash).Scan(&count)
	return count > 0, err
}

// HashProject returns the sha256 dedup key for a raw project.
// Collectors call this before inserting so duplicates are skipped.
func HashProject(source, externalID, title string, issuedAt time.Time) string {
	h := sha256.Sum256([]byte(
		source + "|" + externalID + "|" + title + "|" + issuedAt.Format("2006-01-02"),
	))
	return fmt.Sprintf("%x", h)
}

// NewUUID generates a random UUID v4.
func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
