package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PipelineRun struct {
	ID         string
	Status     string
	Sources    []string
	Counts     map[string]int
	Errors     []string
	Request    map[string]any
	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PipelineRunCompletion struct {
	Status string
	Counts map[string]int
	Errors []string
}

type PipelineRunListFilter struct {
	Status string
	Limit  int
	Cursor string
}

type PipelineRunStore interface {
	Create(ctx context.Context, run PipelineRun) (*PipelineRun, error)
	Complete(ctx context.Context, id string, completion PipelineRunCompletion) error
	List(ctx context.Context, filter PipelineRunListFilter) ([]PipelineRun, string, error)
	Latest(ctx context.Context) (*PipelineRun, error)
}

type sqlPipelineRunStore struct {
	db  *sql.DB
	dsn string
}

func NewPipelineRunStoreWithDSN(db *sql.DB, dsn string) PipelineRunStore {
	return &sqlPipelineRunStore{db: db, dsn: dsn}
}

func (s *sqlPipelineRunStore) Create(ctx context.Context, run PipelineRun) (*PipelineRun, error) {
	now := time.Now().UTC()
	if run.ID == "" {
		run.ID = NewUUID()
	}
	if run.Status == "" {
		run.Status = "running"
	}
	if run.Counts == nil {
		run.Counts = map[string]int{}
	}
	if run.Errors == nil {
		run.Errors = []string{}
	}
	if run.Request == nil {
		run.Request = map[string]any{}
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	run.CreatedAt = now
	run.UpdatedAt = now

	sources, err := json.Marshal(run.Sources)
	if err != nil {
		return nil, err
	}
	counts, err := json.Marshal(run.Counts)
	if err != nil {
		return nil, err
	}
	errorsJSON, err := json.Marshal(run.Errors)
	if err != nil {
		return nil, err
	}
	request, err := json.Marshal(run.Request)
	if err != nil {
		return nil, err
	}
	query := `
		INSERT INTO pipeline_runs (id, status, sources, counts, errors, request, started_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, Rebind(s.dsn, query),
		run.ID, run.Status, string(sources), string(counts), string(errorsJSON), string(request),
		run.StartedAt, run.FinishedAt, run.CreatedAt, run.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *sqlPipelineRunStore) Complete(ctx context.Context, id string, completion PipelineRunCompletion) error {
	if completion.Status == "" {
		completion.Status = "succeeded"
	}
	if completion.Counts == nil {
		completion.Counts = map[string]int{}
	}
	if completion.Errors == nil {
		completion.Errors = []string{}
	}
	counts, err := json.Marshal(completion.Counts)
	if err != nil {
		return err
	}
	errorsJSON, err := json.Marshal(completion.Errors)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	query := `
		UPDATE pipeline_runs
		SET status = ?, counts = ?, errors = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, Rebind(s.dsn, query), completion.Status, string(counts), string(errorsJSON), now, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pipeline run %s not found", id)
	}
	return nil
}

func (s *sqlPipelineRunStore) List(ctx context.Context, filter PipelineRunListFilter) ([]PipelineRun, string, error) {
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
		SELECT id, status, sources, counts, errors, request, started_at, finished_at, created_at, updated_at
		FROM pipeline_runs
	`
	var args []any
	if filter.Status != "" {
		query += " WHERE status = ?"
		args = append(args, filter.Status)
	}
	query += " ORDER BY started_at DESC, created_at DESC, id ASC LIMIT ? OFFSET ?"
	args = append(args, limit+1, offset)
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	runs, err := scanPipelineRuns(rows)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(runs) > limit {
		runs = runs[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return runs, next, nil
}

func (s *sqlPipelineRunStore) Latest(ctx context.Context) (*PipelineRun, error) {
	query := `
		SELECT id, status, sources, counts, errors, request, started_at, finished_at, created_at, updated_at
		FROM pipeline_runs
		ORDER BY started_at DESC, created_at DESC, id ASC
		LIMIT 1
	`
	rows, err := s.db.QueryContext(ctx, Rebind(s.dsn, query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs, err := scanPipelineRuns(rows)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}

func scanPipelineRuns(rows *sql.Rows) ([]PipelineRun, error) {
	var runs []PipelineRun
	for rows.Next() {
		var run PipelineRun
		var sources, counts, errorsJSON, request string
		var finishedAt sql.NullTime
		if err := rows.Scan(&run.ID, &run.Status, &sources, &counts, &errorsJSON, &request, &run.StartedAt, &finishedAt, &run.CreatedAt, &run.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(sources), &run.Sources)
		_ = json.Unmarshal([]byte(counts), &run.Counts)
		_ = json.Unmarshal([]byte(errorsJSON), &run.Errors)
		_ = json.Unmarshal([]byte(request), &run.Request)
		if finishedAt.Valid {
			run.FinishedAt = &finishedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
