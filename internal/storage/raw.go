package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
)

// RawProjectStore is the interface for persisting raw ingest records.
type RawProjectStore interface {
	Insert(ctx context.Context, p *collector.RawProject) error
	ExistsByHash(ctx context.Context, hash string) (bool, error)
}

type sqliteRawStore struct{ db *sql.DB }

// NewRawProjectStore returns a SQLite-backed RawProjectStore.
func NewRawProjectStore(db *sql.DB) RawProjectStore {
	return &sqliteRawStore{db: db}
}

func (s *sqliteRawStore) Insert(ctx context.Context, p *collector.RawProject) error {
	raw, err := json.Marshal(p.RawData)
	if err != nil {
		return fmt.Errorf("marshal raw_data: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO raw_projects (id, source, external_id, raw_data, collected_at, hash)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(hash) DO NOTHING
	`, newUUID(), p.Source, p.ExternalID, string(raw), time.Now().UTC(), p.Hash)
	return err
}

func (s *sqliteRawStore) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM raw_projects WHERE hash = ?`, hash,
	).Scan(&count)
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

// newUUID generates a random UUID v4.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
