package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type OutreachEvent struct {
	ID       string
	LeadID   string
	Contact  string
	Channel  string
	Notes    string
	Outcome  string
	LoggedAt time.Time
}

type OutreachListFilter struct {
	Limit  int
	Cursor string
}

type OutreachStore interface {
	Insert(ctx context.Context, event OutreachEvent) (*OutreachEvent, error)
	ListByLead(ctx context.Context, leadID string, filter OutreachListFilter) ([]OutreachEvent, string, error)
}

type sqlOutreachStore struct {
	db  *sql.DB
	dsn string
}

func NewOutreachStoreWithDSN(db *sql.DB, dsn string) OutreachStore {
	return &sqlOutreachStore{db: db, dsn: dsn}
}

func (s *sqlOutreachStore) Insert(ctx context.Context, event OutreachEvent) (*OutreachEvent, error) {
	if strings.TrimSpace(event.LeadID) == "" {
		return nil, fmt.Errorf("lead_id is required")
	}
	if strings.TrimSpace(event.Channel) == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if event.ID == "" {
		event.ID = NewUUID()
	}
	if event.LoggedAt.IsZero() {
		event.LoggedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO outreach_log (id, lead_id, contact, channel, notes, outcome, logged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		event.ID, event.LeadID, event.Contact, event.Channel, event.Notes, event.Outcome, event.LoggedAt,
	); err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *sqlOutreachStore) ListByLead(ctx context.Context, leadID string, filter OutreachListFilter) ([]OutreachEvent, string, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := 0
	if strings.TrimSpace(filter.Cursor) != "" {
		parsed, err := strconv.Atoi(filter.Cursor)
		if err != nil || parsed < 0 {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		offset = parsed
	}

	query := `
		SELECT id, lead_id, contact, channel, notes, outcome, logged_at
		FROM outreach_log
		WHERE lead_id = ?
		ORDER BY logged_at DESC, id ASC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), leadID, limit+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var events []OutreachEvent
	for rows.Next() {
		var event OutreachEvent
		if err := rows.Scan(&event.ID, &event.LeadID, &event.Contact, &event.Channel, &event.Notes, &event.Outcome, &event.LoggedAt); err != nil {
			return nil, "", err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(events) > limit {
		events = events[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return events, next, nil
}
