package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrLeadNotFound = errors.New("lead not found")

// Lead is a fully enriched, scored project ready for the sales team.
type Lead struct {
	ID                      string
	RawProjectID            string
	RawInputID              string
	Source                  string
	Title                   string
	Location                string
	ProjectValue            int64
	GeneralContractor       string
	Applicant               string // raw applicant from permit (may include phone/contact)
	Contractor              string // raw contractor from permit (may include phone/contact)
	SourceURL               string // direct link to the source document (PDF, page, etc.)
	ProjectType             string
	EstimatedCrewSize       int
	EstimatedDurationMonths int
	OutOfTownCrewLikely     bool
	PriorityScore           int
	PriorityReason          string
	Rationale               string
	SuggestedOutreachTiming string
	Notes                   string
	Status                  string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type LeadListFilter struct {
	Status   string
	Source   string
	MinScore int
	Query    string
	Limit    int
	Cursor   string
}

type LeadPatch struct {
	Status *string
	Notes  *string
}

type LeadPatchResult struct {
	Lead          Lead
	ChangedFields []string
	UpdatedAt     time.Time
}

// LeadStore is the interface for persisting and querying enriched leads.
type LeadStore interface {
	Insert(ctx context.Context, l *Lead) error
	ListNew(ctx context.Context) ([]Lead, error)
	ListFiltered(ctx context.Context, filter LeadListFilter) ([]Lead, string, error)
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateOperatorFields(ctx context.Context, id string, patch LeadPatch) (*LeadPatchResult, error)
	ListForDigest(ctx context.Context) ([]Lead, error)
	GetByID(ctx context.Context, id string) (*Lead, error)
}

type sqliteLeadStore struct {
	db  *sql.DB
	dsn string
}

// NewLeadStore returns a LeadStore.
func NewLeadStore(db *sql.DB) LeadStore {
	// We don't have the DSN here easily, but we can't easily change the signature
	// if it's used elsewhere. However, NewLeadStore is only used in main.go
	// where we have the DSN. Let's see if we can find a way to get DSN from db
	// or just change the signature.
	return &sqliteLeadStore{db: db}
}

// NewLeadStoreWithDSN returns a LeadStore that knows its DSN for rebinding.
func NewLeadStoreWithDSN(db *sql.DB, dsn string) LeadStore {
	return &sqliteLeadStore{db: db, dsn: dsn}
}

func (s *sqliteLeadStore) Insert(ctx context.Context, l *Lead) error {
	now := time.Now().UTC()
	if l.ID == "" {
		l.ID = NewUUID()
	}
	if l.Status == "" {
		l.Status = "new"
	}
	l.CreatedAt = now
	l.UpdatedAt = now

	query := `
		INSERT INTO leads (
			id, raw_project_id, raw_input_id, source, title, location, project_value,
			general_contractor, applicant, contractor, source_url, project_type,
			estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
			priority_score, priority_reason, rationale, suggested_outreach_timing,
			notes, status, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`
	var rawProjectID any
	if l.RawProjectID != "" {
		rawProjectID = l.RawProjectID
	}
	var rawInputID any
	if l.RawInputID != "" {
		rawInputID = l.RawInputID
	}
	_, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		l.ID, rawProjectID, rawInputID, l.Source, l.Title, l.Location, l.ProjectValue,
		l.GeneralContractor, l.Applicant, l.Contractor, l.SourceURL, l.ProjectType,
		l.EstimatedCrewSize, l.EstimatedDurationMonths, l.OutOfTownCrewLikely,
		l.PriorityScore, l.PriorityReason, l.Rationale, l.SuggestedOutreachTiming,
		l.Notes, l.Status, now, now,
	)
	return err
}

func (s *sqliteLeadStore) ListNew(ctx context.Context) ([]Lead, error) {
	query := `
		SELECT id, raw_project_id, raw_input_id, source, title, location, project_value,
		       general_contractor, applicant, contractor, source_url, project_type,
		       estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
		       priority_score, priority_reason, rationale, suggested_outreach_timing,
		       notes, status, created_at, updated_at
		FROM leads
		WHERE status = 'new'
		ORDER BY priority_score DESC, created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		var rawProjectID sql.NullString
		var rawInputID sql.NullString
		if err := rows.Scan(
			&l.ID, &rawProjectID, &rawInputID, &l.Source, &l.Title, &l.Location, &l.ProjectValue,
			&l.GeneralContractor, &l.Applicant, &l.Contractor, &l.SourceURL, &l.ProjectType,
			&l.EstimatedCrewSize, &l.EstimatedDurationMonths, &l.OutOfTownCrewLikely,
			&l.PriorityScore, &l.PriorityReason, &l.Rationale, &l.SuggestedOutreachTiming,
			&l.Notes, &l.Status, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		l.RawProjectID = rawProjectID.String
		l.RawInputID = rawInputID.String
		leads = append(leads, l)
	}
	return leads, rows.Err()
}

func (s *sqliteLeadStore) ListFiltered(ctx context.Context, filter LeadListFilter) ([]Lead, string, error) {
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
		SELECT id, raw_project_id, raw_input_id, source, title, location, project_value,
		       general_contractor, applicant, contractor, source_url, project_type,
		       estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
		       priority_score, priority_reason, rationale, suggested_outreach_timing,
		       notes, status, created_at, updated_at
		FROM leads
	`
	var where []string
	var args []any
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Source != "" {
		where = append(where, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.MinScore > 0 {
		where = append(where, "priority_score >= ?")
		args = append(args, filter.MinScore)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(location) LIKE ? OR LOWER(priority_reason) LIKE ?)")
		pattern := "%" + strings.ToLower(q) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY priority_score DESC, created_at DESC, id ASC LIMIT ? OFFSET ?"
	args = append(args, limit+1, offset)

	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	leads, err := scanLeads(rows)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(leads) > limit {
		leads = leads[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return leads, next, nil
}

func (s *sqliteLeadStore) ListForDigest(ctx context.Context) ([]Lead, error) {
	query := `
		SELECT id, raw_project_id, raw_input_id, source, title, location, project_value,
		       general_contractor, applicant, contractor, source_url, project_type,
		       estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
		       priority_score, priority_reason, rationale, suggested_outreach_timing,
		       notes, status, created_at, updated_at
		FROM leads
		WHERE (status = 'notified' OR status = 'new')
		  AND created_at >= ?
		ORDER BY priority_score DESC, created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		var rawProjectID sql.NullString
		var rawInputID sql.NullString
		if err := rows.Scan(
			&l.ID, &rawProjectID, &rawInputID, &l.Source, &l.Title, &l.Location, &l.ProjectValue,
			&l.GeneralContractor, &l.Applicant, &l.Contractor, &l.SourceURL, &l.ProjectType,
			&l.EstimatedCrewSize, &l.EstimatedDurationMonths, &l.OutOfTownCrewLikely,
			&l.PriorityScore, &l.PriorityReason, &l.Rationale, &l.SuggestedOutreachTiming,
			&l.Notes, &l.Status, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		l.RawProjectID = rawProjectID.String
		l.RawInputID = rawInputID.String
		leads = append(leads, l)
	}
	return leads, rows.Err()
}

func (s *sqliteLeadStore) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE leads SET status = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		status, time.Now().UTC(), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrLeadNotFound, id)
	}
	return nil
}

func (s *sqliteLeadStore) UpdateOperatorFields(ctx context.Context, id string, patch LeadPatch) (*LeadPatchResult, error) {
	var assignments []string
	var args []any
	var changed []string
	if patch.Status != nil {
		assignments = append(assignments, "status = ?")
		args = append(args, *patch.Status)
		changed = append(changed, "status")
	}
	if patch.Notes != nil {
		assignments = append(assignments, "notes = ?")
		args = append(args, *patch.Notes)
		changed = append(changed, "notes")
	}
	if len(assignments) == 0 {
		lead, err := s.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if lead == nil {
			return nil, fmt.Errorf("%w: %s", ErrLeadNotFound, id)
		}
		return &LeadPatchResult{Lead: *lead, UpdatedAt: lead.UpdatedAt}, nil
	}

	now := time.Now().UTC()
	assignments = append(assignments, "updated_at = ?")
	args = append(args, now, id)
	query := "UPDATE leads SET " + strings.Join(assignments, ", ") + " WHERE id = ?"
	res, err := s.db.ExecContext(ctx, Rebind(s.dsn, query), args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("%w: %s", ErrLeadNotFound, id)
	}

	lead, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, fmt.Errorf("%w: %s", ErrLeadNotFound, id)
	}
	return &LeadPatchResult{Lead: *lead, ChangedFields: changed, UpdatedAt: now}, nil
}

func (s *sqliteLeadStore) GetByID(ctx context.Context, id string) (*Lead, error) {
	query := `
		SELECT id, raw_project_id, raw_input_id, source, title, location, project_value,
		       general_contractor, applicant, contractor, source_url, project_type,
		       estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
		       priority_score, priority_reason, rationale, suggested_outreach_timing,
		       notes, status, created_at, updated_at
		FROM leads
		WHERE id = ?
	`
	var l Lead
	var rawProjectID sql.NullString
	var rawInputID sql.NullString
	err := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), id).Scan(
		&l.ID, &rawProjectID, &rawInputID, &l.Source, &l.Title, &l.Location, &l.ProjectValue,
		&l.GeneralContractor, &l.Applicant, &l.Contractor, &l.SourceURL, &l.ProjectType,
		&l.EstimatedCrewSize, &l.EstimatedDurationMonths, &l.OutOfTownCrewLikely,
		&l.PriorityScore, &l.PriorityReason, &l.Rationale, &l.SuggestedOutreachTiming,
		&l.Notes, &l.Status, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	l.RawProjectID = rawProjectID.String
	l.RawInputID = rawInputID.String
	return &l, nil
}

func scanLeads(rows *sql.Rows) ([]Lead, error) {
	var leads []Lead
	for rows.Next() {
		var l Lead
		var rawProjectID sql.NullString
		var rawInputID sql.NullString
		if err := rows.Scan(
			&l.ID, &rawProjectID, &rawInputID, &l.Source, &l.Title, &l.Location, &l.ProjectValue,
			&l.GeneralContractor, &l.Applicant, &l.Contractor, &l.SourceURL, &l.ProjectType,
			&l.EstimatedCrewSize, &l.EstimatedDurationMonths, &l.OutOfTownCrewLikely,
			&l.PriorityScore, &l.PriorityReason, &l.Rationale, &l.SuggestedOutreachTiming,
			&l.Notes, &l.Status, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		l.RawProjectID = rawProjectID.String
		l.RawInputID = rawInputID.String
		leads = append(leads, l)
	}
	return leads, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
