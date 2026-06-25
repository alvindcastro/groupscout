package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

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

// LeadStore is the interface for persisting and querying enriched leads.
type LeadStore interface {
	Insert(ctx context.Context, l *Lead) error
	ExistsBySourceTitle(ctx context.Context, source, title string) (bool, error)
	ListNew(ctx context.Context) ([]Lead, error)
	ListDeliveryCandidates(ctx context.Context, limit int) ([]Lead, error)
	UpdateStatus(ctx context.Context, id, status string) error
	ListForDigest(ctx context.Context) ([]Lead, error)
	GetByID(ctx context.Context, id string) (*Lead, error)
}

type sqliteLeadStore struct {
	db  *sql.DB
	dsn string
}

// NewLeadStoreWithDSN returns a LeadStore that knows its DSN for rebinding.
func NewLeadStoreWithDSN(db *sql.DB, dsn string) LeadStore {
	return &sqliteLeadStore{db: db, dsn: dsn}
}

// leadColumns is the canonical column list, in struct-field order, shared by
// every SELECT so the projection always matches scanLead.
const leadColumns = `id, raw_project_id, raw_input_id, source, title, location, project_value,
	general_contractor, applicant, contractor, source_url, project_type,
	estimated_crew_size, estimated_duration_months, out_of_town_crew_likely,
	priority_score, priority_reason, rationale, suggested_outreach_timing,
	notes, status, created_at, updated_at`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanLead reads one leadColumns row into a Lead.
func scanLead(s rowScanner) (Lead, error) {
	var l Lead
	var rawProjectID, rawInputID sql.NullString
	if err := s.Scan(
		&l.ID, &rawProjectID, &rawInputID, &l.Source, &l.Title, &l.Location, &l.ProjectValue,
		&l.GeneralContractor, &l.Applicant, &l.Contractor, &l.SourceURL, &l.ProjectType,
		&l.EstimatedCrewSize, &l.EstimatedDurationMonths, &l.OutOfTownCrewLikely,
		&l.PriorityScore, &l.PriorityReason, &l.Rationale, &l.SuggestedOutreachTiming,
		&l.Notes, &l.Status, &l.CreatedAt, &l.UpdatedAt,
	); err != nil {
		return Lead{}, err
	}
	l.RawProjectID = rawProjectID.String
	l.RawInputID = rawInputID.String
	return l, nil
}

// scanLeads drains a result set of leadColumns rows, closing it when done.
func scanLeads(rows *sql.Rows) ([]Lead, error) {
	defer rows.Close()
	var leads []Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		leads = append(leads, l)
	}
	return leads, rows.Err()
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

func (s *sqliteLeadStore) ExistsBySourceTitle(ctx context.Context, source, title string) (bool, error) {
	if source == "" || title == "" {
		return false, nil
	}
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM leads WHERE source = ? AND title = ?)`
	err := s.db.QueryRowContext(ctx, Rebind(s.dsn, query), source, title).Scan(&exists)
	return exists, err
}

func (s *sqliteLeadStore) ListNew(ctx context.Context) ([]Lead, error) {
	query := `SELECT ` + leadColumns + `
		FROM leads
		WHERE status = 'new'
		ORDER BY priority_score DESC, created_at DESC`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query))
	if err != nil {
		return nil, err
	}
	return scanLeads(rows)
}

func (s *sqliteLeadStore) ListForDigest(ctx context.Context) ([]Lead, error) {
	query := `SELECT ` + leadColumns + `
		FROM leads
		WHERE (status = 'notified' OR status = 'new')
		  AND created_at >= ?
		ORDER BY priority_score DESC, created_at DESC`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	return scanLeads(rows)
}

func (s *sqliteLeadStore) ListDeliveryCandidates(ctx context.Context, limit int) ([]Lead, error) {
	if limit <= 0 {
		limit = 1
	}
	query := `SELECT ` + leadColumns + `
		FROM leads
		WHERE status IN ('new', 'notified')
		  AND id NOT IN (SELECT lead_id FROM lead_deliveries WHERE status = 'sent' AND lead_id IS NOT NULL)
		ORDER BY
		  CASE WHEN status = 'new' THEN 0 ELSE 1 END,
		  priority_score DESC,
		  created_at DESC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), limit)
	if err != nil {
		return nil, err
	}
	return scanLeads(rows)
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
		return fmt.Errorf("lead %s not found", id)
	}
	return nil
}

func (s *sqliteLeadStore) GetByID(ctx context.Context, id string) (*Lead, error) {
	query := `SELECT ` + leadColumns + ` FROM leads WHERE id = ?`
	l, err := scanLead(s.db.QueryRowContext(ctx, Rebind(s.dsn, query), id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
