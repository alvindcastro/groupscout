package auditretention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Store is the storage behavior required by the retention worker.
type Store interface {
	PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error)
}

// Policy controls which raw audit inputs are eligible for retention cleanup.
type Policy struct {
	RetentionDays int
	Interval      time.Duration
	RunOnStart    bool
	Now           func() time.Time
}

// Result describes one purge attempt.
type Result struct {
	Cutoff    time.Time `json:"cutoff"`
	Deleted   int64     `json:"deleted"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Worker periodically removes unreferenced raw audit inputs older than Policy.RetentionDays.
type Worker struct {
	store  Store
	policy Policy
	log    *slog.Logger
}

// New returns a retention worker after validating the configured policy.
func New(store Store, policy Policy, log *slog.Logger) (*Worker, error) {
	if store == nil {
		return nil, errors.New("audit retention store is required")
	}
	if policy.RetentionDays <= 0 {
		return nil, fmt.Errorf("audit retention days must be positive: %d", policy.RetentionDays)
	}
	if policy.Interval <= 0 {
		return nil, fmt.Errorf("audit retention interval must be positive: %s", policy.Interval)
	}
	if policy.Now == nil {
		policy.Now = func() time.Time { return time.Now().UTC() }
	}
	if log == nil {
		log = slog.Default()
	}
	return &Worker{store: store, policy: policy, log: log}, nil
}

// PurgeOnce runs one retention purge and returns machine-readable details.
func (w *Worker) PurgeOnce(ctx context.Context) (Result, error) {
	started := w.policy.Now().UTC()
	cutoff := started.AddDate(0, 0, -w.policy.RetentionDays)
	deleted, err := w.store.PurgeOlderThan(ctx, cutoff)
	ended := w.policy.Now().UTC()
	result := Result{
		Cutoff:    cutoff,
		Deleted:   deleted,
		StartedAt: started,
		EndedAt:   ended,
	}
	if err != nil {
		return result, fmt.Errorf("audit retention purge: %w", err)
	}
	return result, nil
}

// Run starts the retention loop and blocks until ctx is canceled.
func (w *Worker) Run(ctx context.Context) {
	if w.policy.RunOnStart {
		w.runAndLog(ctx)
	}

	ticker := time.NewTicker(w.policy.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("audit retention worker stopped", "error", ctx.Err())
			return
		case <-ticker.C:
			w.runAndLog(ctx)
		}
	}
}

func (w *Worker) runAndLog(ctx context.Context) {
	result, err := w.PurgeOnce(ctx)
	if err != nil {
		w.log.Error("audit retention purge failed", "error", err, "cutoff", result.Cutoff)
		return
	}
	w.log.Info("audit retention purge complete", "deleted", result.Deleted, "cutoff", result.Cutoff)
}
