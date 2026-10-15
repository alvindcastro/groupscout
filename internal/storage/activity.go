package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Activity is when the lead cadence last completed and when the newest lead
// was created. The server seeds its monitoring metrics from it at startup, so
// a restart does not hide a missed cadence run (IaC Phase 9).
type Activity struct {
	CadenceRunAt  time.Time // zero if no cadence run is recorded
	LeadCreatedAt time.Time // zero if there are no leads
}

// LoadActivity reads Activity from Postgres. SQLite (local development) is
// not supported and returns zero times.
func LoadActivity(ctx context.Context, db *sql.DB, dsn string) (Activity, error) {
	var a Activity
	if DriverName(dsn) != "pgx" {
		return a, nil
	}
	// A cadence run that reached the delivery step writes a lead_deliveries
	// row ("sent" or "no_eligible_lead"); "failed" rows are runs that broke.
	var run, lead sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT
			(SELECT MAX(updated_at) FROM lead_deliveries WHERE status <> 'failed'),
			(SELECT MAX(created_at) FROM leads)
	`).Scan(&run, &lead)
	if err != nil {
		return a, fmt.Errorf("load activity: %w", err)
	}
	a.CadenceRunAt = run.Time
	a.LeadCreatedAt = lead.Time
	return a, nil
}
