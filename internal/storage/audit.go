package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RawInput represents the audit record of raw data collected.
type RawInput struct {
	ID            uuid.UUID
	Hash          string
	PayloadType   string
	Payload       []byte
	SourceURL     string
	CollectorName string
	CreatedAt     time.Time
}

// AuditStore handles the storage and retrieval of raw audit inputs.
type AuditStore interface {
	Store(ctx context.Context, raw RawInput) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RawInput, error)
	GetByHash(ctx context.Context, hash string) (*RawInput, error)
	ExistsByHash(ctx context.Context, hash string) (bool, error)
}

type sqlAuditStore struct {
	db  *sql.DB
	dsn string
}

// NewAuditStore returns a new instance of AuditStore.
func NewAuditStore(db *sql.DB) AuditStore {
	return &sqlAuditStore{db: db}
}

// NewAuditStoreWithDSN returns a new instance of AuditStore with DSN for rebind.
func NewAuditStoreWithDSN(db *sql.DB, dsn string) AuditStore {
	return &sqlAuditStore{db: db, dsn: dsn}
}

func (s *sqlAuditStore) Store(ctx context.Context, raw RawInput) (uuid.UUID, error) {
	if raw.ID == uuid.Nil {
		raw.ID = uuid.New()
	}
	if raw.CreatedAt.IsZero() {
		raw.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO raw_inputs (id, hash, payload_type, payload, source_url, collector_name, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (hash) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		raw.ID, raw.Hash, raw.PayloadType, raw.Payload, raw.SourceURL, raw.CollectorName, raw.CreatedAt)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert raw_input: %w", err)
	}

	// Fetch the actual ID (it might have been a conflict)
	query = `SELECT id FROM raw_inputs WHERE hash = ?`
	var id uuid.UUID
	err = s.db.QueryRowContext(ctx, Rebind(s.dsn, query), raw.Hash).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("select raw_input id: %w", err)
	}

	return id, nil
}

func (s *sqlAuditStore) GetByID(ctx context.Context, id uuid.UUID) (*RawInput, error) {
	query := `SELECT id, hash, payload_type, payload, source_url, collector_name, created_at FROM raw_inputs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), id)
	var r RawInput
	err := row.Scan(&r.ID, &r.Hash, &r.PayloadType, &r.Payload, &r.SourceURL, &r.CollectorName, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get raw_input by id: %w", err)
	}
	return &r, nil
}

func (s *sqlAuditStore) GetByHash(ctx context.Context, hash string) (*RawInput, error) {
	query := `SELECT id, hash, payload_type, payload, source_url, collector_name, created_at FROM raw_inputs WHERE hash = ?`
	row := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), hash)
	var r RawInput
	err := row.Scan(&r.ID, &r.Hash, &r.PayloadType, &r.Payload, &r.SourceURL, &r.CollectorName, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get raw_input by hash: %w", err)
	}
	return &r, nil
}

func (s *sqlAuditStore) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM raw_inputs WHERE hash = ?)`
	var exists bool
	err := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), hash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by hash: %w", err)
	}
	return exists, nil
}
