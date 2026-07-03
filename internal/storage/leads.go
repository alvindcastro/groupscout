package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
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

// SourceAttribution summarizes source yield and action outcomes in a time window.
type SourceAttribution struct {
	Source  string
	Leads   int
	Claimed int
	Won     int
	HitRate float64
}

// DemandBucket groups actionable demand signals by lead-created week and source.
type DemandBucket struct {
	WeekStart         time.Time
	Source            string
	Leads             int
	EstimatedCrewSize int
}

// LeadStore is the interface for persisting and querying enriched leads.
type LeadStore interface {
	Insert(ctx context.Context, l *Lead) error
	ExistsBySourceTitle(ctx context.Context, source, title string) (bool, error)
	ListNew(ctx context.Context) ([]Lead, error)
	ListDeliveryCandidates(ctx context.Context, limit int) ([]Lead, error)
	UpdateStatus(ctx context.Context, id, status string) error
	ListForDigest(ctx context.Context) ([]Lead, error)
	SourceAttribution(ctx context.Context, since time.Time) ([]SourceAttribution, error)
	DemandDensityByWeek(ctx context.Context, since time.Time) ([]DemandBucket, error)
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

func (s *sqliteLeadStore) SourceAttribution(ctx context.Context, since time.Time) ([]SourceAttribution, error) {
	query := `
		SELECT
			COALESCE(NULLIF(source, ''), 'unknown') AS source_name,
			COUNT(*) AS leads,
			SUM(CASE WHEN status = 'claimed' THEN 1 ELSE 0 END) AS claimed,
			SUM(CASE WHEN status = 'won' THEN 1 ELSE 0 END) AS won
		FROM leads
		WHERE created_at >= ?
		GROUP BY source_name
		ORDER BY leads DESC, source_name ASC`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SourceAttribution
	for rows.Next() {
		var row SourceAttribution
		if err := rows.Scan(&row.Source, &row.Leads, &row.Claimed, &row.Won); err != nil {
			return nil, err
		}
		if row.Leads > 0 {
			row.HitRate = float64(row.Claimed+row.Won) / float64(row.Leads) * 100
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *sqliteLeadStore) DemandDensityByWeek(ctx context.Context, since time.Time) ([]DemandBucket, error) {
	query := `
		SELECT COALESCE(NULLIF(source, ''), 'unknown'), created_at, estimated_crew_size
		FROM leads
		WHERE created_at >= ?
		  AND status NOT IN ('dismissed', 'lost', 'skipped')
		ORDER BY created_at ASC, source ASC`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type key struct {
		week   time.Time
		source string
	}
	buckets := map[key]DemandBucket{}
	for rows.Next() {
		var source string
		var createdAt time.Time
		var crew int
		if err := rows.Scan(&source, &createdAt, &crew); err != nil {
			return nil, err
		}
		k := key{week: weekStartUTC(createdAt), source: source}
		b := buckets[k]
		if b.Source == "" {
			b = DemandBucket{WeekStart: k.week, Source: source}
		}
		b.Leads++
		b.EstimatedCrewSize += crew
		buckets[k] = b
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]DemandBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, bucket)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].WeekStart.Equal(out[j].WeekStart) {
			return out[i].WeekStart.Before(out[j].WeekStart)
		}
		return out[i].Source < out[j].Source
	})
	return out, nil
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

func weekStartUTC(t time.Time) time.Time {
	t = t.UTC()
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}
