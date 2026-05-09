package storage

import (
	"context"
	"database/sql"
)

type StatsFilter struct {
	Window string
}

type StatsSummary struct {
	Window     string
	ByStatus   map[string]int
	BySource   map[string]int
	ScoreBands map[string]int
	ByOwner    map[string]int
	ByWeek     map[string]int
	ByOutcome  map[string]int
}

type StatsStore interface {
	Summary(ctx context.Context, filter StatsFilter) (*StatsSummary, error)
}

type sqlStatsStore struct {
	db  *sql.DB
	dsn string
}

func NewStatsStoreWithDSN(db *sql.DB, dsn string) StatsStore {
	return &sqlStatsStore{db: db, dsn: dsn}
}

func (s *sqlStatsStore) Summary(ctx context.Context, filter StatsFilter) (*StatsSummary, error) {
	if filter.Window == "" {
		filter.Window = "30d"
	}
	summary := &StatsSummary{
		Window:     filter.Window,
		ByStatus:   map[string]int{},
		BySource:   map[string]int{},
		ScoreBands: map[string]int{},
		ByOwner:    map[string]int{},
		ByWeek:     map[string]int{},
		ByOutcome:  map[string]int{},
	}
	if err := scanCounts(ctx, s.db, s.dsn, `SELECT COALESCE(status, 'unknown'), COUNT(*) FROM leads GROUP BY COALESCE(status, 'unknown')`, summary.ByStatus); err != nil {
		return nil, err
	}
	if err := scanCounts(ctx, s.db, s.dsn, `SELECT COALESCE(source, 'unknown'), COUNT(*) FROM leads GROUP BY COALESCE(source, 'unknown')`, summary.BySource); err != nil {
		return nil, err
	}
	if err := scanCounts(ctx, s.db, s.dsn, `SELECT COALESCE(NULLIF(owner, ''), 'unassigned'), COUNT(*) FROM leads GROUP BY COALESCE(NULLIF(owner, ''), 'unassigned')`, summary.ByOwner); err != nil {
		return nil, err
	}
	if err := scanCounts(ctx, s.db, s.dsn, `SELECT substr(CAST(created_at AS TEXT), 1, 10), COUNT(*) FROM leads GROUP BY substr(CAST(created_at AS TEXT), 1, 10)`, summary.ByWeek); err != nil {
		if DriverName(s.dsn) == "pgx" {
			if err := scanCounts(ctx, s.db, s.dsn, `SELECT to_char(created_at, 'IYYY-"W"IW'), COUNT(*) FROM leads GROUP BY to_char(created_at, 'IYYY-"W"IW')`, summary.ByWeek); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	if err := scanCounts(ctx, s.db, s.dsn, `SELECT COALESCE(NULLIF(outcome, ''), 'unknown'), COUNT(*) FROM outreach_log GROUP BY COALESCE(NULLIF(outcome, ''), 'unknown')`, summary.ByOutcome); err != nil {
		return nil, err
	}
	if err := s.scanScoreBands(ctx, summary.ScoreBands); err != nil {
		return nil, err
	}
	return summary, nil
}

func scanCounts(ctx context.Context, db *sql.DB, dsn, query string, dest map[string]int) error {
	rows, err := db.QueryContext(ctx, Rebind(dsn, query))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return err
		}
		dest[key] = count
	}
	return rows.Err()
}

func (s *sqlStatsStore) scanScoreBands(ctx context.Context, dest map[string]int) error {
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, `SELECT priority_score FROM leads`))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var score int
		if err := rows.Scan(&score); err != nil {
			return err
		}
		switch {
		case score >= 8:
			dest["high"]++
		case score >= 4:
			dest["medium"]++
		default:
			dest["low"]++
		}
	}
	return rows.Err()
}
