package storage

import (
	"context"
	"database/sql"
	"time"
)

// LeadDelivery records the result for one scheduled lead-delivery cadence.
type LeadDelivery struct {
	ID             string
	IdempotencyKey string
	ScheduleKey    string
	LeadID         string
	Channel        string
	Status         string
	Result         string
	SentAt         time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DeliveryStore persists scheduled delivery attempts and coordinates run locks.
type DeliveryStore interface {
	GetByIdempotencyKey(ctx context.Context, key string) (*LeadDelivery, error)
	UpsertResult(ctx context.Context, delivery LeadDelivery) error
	TryAcquireRunLock(ctx context.Context, name, owner string, ttl time.Duration) (bool, error)
	ReleaseRunLock(ctx context.Context, name, owner string) error
}

type sqlDeliveryStore struct {
	db  *sql.DB
	dsn string
}

func NewDeliveryStore(db *sql.DB, dsn string) DeliveryStore {
	return &sqlDeliveryStore{db: db, dsn: dsn}
}

func (s *sqlDeliveryStore) GetByIdempotencyKey(ctx context.Context, key string) (*LeadDelivery, error) {
	query := `
		SELECT id, idempotency_key, schedule_key, lead_id, channel, status, result,
		       sent_at, created_at, updated_at
		FROM lead_deliveries
		WHERE idempotency_key = ?
	`
	var d LeadDelivery
	var leadID sql.NullString
	var sentAt sql.NullTime
	err := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), key).Scan(
		&d.ID, &d.IdempotencyKey, &d.ScheduleKey, &leadID, &d.Channel, &d.Status, &d.Result,
		&sentAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	d.LeadID = leadID.String
	d.SentAt = sentAt.Time
	return &d, nil
}

func (s *sqlDeliveryStore) UpsertResult(ctx context.Context, d LeadDelivery) error {
	now := time.Now().UTC()
	if d.ID == "" {
		d.ID = NewUUID()
	}
	if d.Channel == "" {
		d.Channel = "slack"
	}
	if d.Status == "" {
		d.Status = "unknown"
	}
	if d.Result == "" {
		d.Result = d.Status
	}

	var leadID any
	if d.LeadID != "" {
		leadID = d.LeadID
	}
	var sentAt any
	if d.Status == "sent" {
		if d.SentAt.IsZero() {
			d.SentAt = now
		}
		sentAt = d.SentAt
	}

	query := `
		INSERT INTO lead_deliveries (
			id, idempotency_key, schedule_key, lead_id, channel, status, result,
			sent_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(idempotency_key) DO UPDATE SET
			schedule_key = excluded.schedule_key,
			lead_id = excluded.lead_id,
			channel = excluded.channel,
			status = excluded.status,
			result = excluded.result,
			sent_at = excluded.sent_at,
			updated_at = excluded.updated_at
		WHERE lead_deliveries.status <> 'sent'
	`
	_, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		d.ID, d.IdempotencyKey, d.ScheduleKey, leadID, d.Channel, d.Status, d.Result,
		sentAt, now, now,
	)
	return err
}

func (s *sqlDeliveryStore) TryAcquireRunLock(ctx context.Context, name, owner string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	query := `
		INSERT INTO delivery_locks (name, owner, acquired_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			owner = excluded.owner,
			acquired_at = excluded.acquired_at,
			expires_at = excluded.expires_at
		WHERE delivery_locks.expires_at <= ?
	`
	res, err := s.db.ExecContext(ctx, Rebind(s.dsn, query), name, owner, now, expiresAt, now)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *sqlDeliveryStore) ReleaseRunLock(ctx context.Context, name, owner string) error {
	query := `DELETE FROM delivery_locks WHERE name = ? AND owner = ?`
	_, err := s.db.ExecContext(ctx, Rebind(s.dsn, query), name, owner)
	return err
}
