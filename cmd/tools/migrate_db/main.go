package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type args struct {
	sqliteDSN   string
	postgresDSN string
	dryRun      bool
}

func parseArgs(osArgs []string) (args, error) {
	var res args
	for i := 0; i < len(osArgs); i++ {
		switch osArgs[i] {
		case "--sqlite":
			if i+1 < len(osArgs) {
				res.sqliteDSN = osArgs[i+1]
				i++
			}
		case "--postgres":
			if i+1 < len(osArgs) {
				res.postgresDSN = osArgs[i+1]
				i++
			}
		case "--dry-run":
			res.dryRun = true
		}
	}
	if res.sqliteDSN == "" {
		return res, errors.New("missing --sqlite")
	}
	if res.postgresDSN == "" {
		return res, errors.New("missing --postgres")
	}
	return res, nil
}

func main() {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Usage: migrate_to_postgres --sqlite groupscout.db --postgres \"postgres://...\" [--dry-run]\nError: %v\n", err)
		os.Exit(1)
	}

	sqliteDB, err := sql.Open("sqlite", args.sqliteDSN)
	if err != nil {
		log.Fatalf("failed to open sqlite: %v", err)
	}
	defer sqliteDB.Close()

	pgDB, err := sql.Open("pgx", args.postgresDSN)
	if err != nil {
		log.Fatalf("failed to open postgres: %v", err)
	}
	defer pgDB.Close()

	if err := migrateTable(sqliteDB, pgDB, "raw_projects", migrateRawProject, args.dryRun); err != nil {
		log.Fatalf("failed to migrate raw_projects: %v", err)
	}
	if err := migrateTable(sqliteDB, pgDB, "leads", migrateLead, args.dryRun); err != nil {
		log.Fatalf("failed to migrate leads: %v", err)
	}
	if err := migrateTable(sqliteDB, pgDB, "outreach_log", migrateOutreachLog, args.dryRun); err != nil {
		log.Fatalf("failed to migrate outreach_log: %v", err)
	}
	if err := migrateTable(sqliteDB, pgDB, "lead_embeddings_sqlite", migrateLeadEmbedding, args.dryRun); err != nil {
		log.Fatalf("failed to migrate lead_embeddings: %v", err)
	}
}

func migrateTable(sqliteDB, pgDB *sql.DB, tableName string, migrateFn func(*sql.Rows) ([]any, error), dryRun bool) error {
	targetTable := tableName
	conflictKey := "id"
	if tableName == "lead_embeddings_sqlite" {
		targetTable = "lead_embeddings"
		conflictKey = "lead_id"
	}

	rows, err := sqliteDB.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO NOTHING",
		targetTable, strings.Join(cols, ", "), strings.Join(placeholders, ", "), conflictKey)

	count := 0
	batchSize := 100
	var batch [][]any

	for rows.Next() {
		vals, err := migrateFn(rows)
		if err != nil {
			return err
		}
		count++
		if !dryRun {
			batch = append(batch, vals)
			if len(batch) >= batchSize {
				if err := execBatch(pgDB, query, batch); err != nil {
					return err
				}
				batch = nil
			}
		}
	}

	if !dryRun && len(batch) > 0 {
		if err := execBatch(pgDB, query, batch); err != nil {
			return err
		}
	}

	fmt.Printf("Migrated %d %s\n", count, tableName)

	// Comparison
	var pgCount int
	if !dryRun {
		err = pgDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", targetTable)).Scan(&pgCount)
		if err != nil {
			return err
		}
	}
	var sqliteCount int
	err = sqliteDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&sqliteCount)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("SQLite: %d, Postgres: (dry run)\n", sqliteCount)
	} else {
		fmt.Printf("SQLite: %d, Postgres: %d\n", sqliteCount, pgCount)
	}

	return nil
}

func migrateLeadEmbedding(rows *sql.Rows) ([]any, error) {
	var leadID, model, embeddingStr, createdAtStr string
	if err := rows.Scan(&leadID, &model, &embeddingStr, &createdAtStr); err != nil {
		return nil, err
	}
	createdAt, _ := parseTime(createdAtStr)
	return []any{leadID, model, embeddingStr, createdAt}, nil
}

func execBatch(db *sql.DB, query string, batch [][]any) error {
	for _, vals := range batch {
		if _, err := db.Exec(query, vals...); err != nil {
			return err
		}
	}
	return nil
}

func parseTime(s string) (time.Time, error) {
	// SQLite DATETIME can be "2026-04-05 18:53:00" or RFC3339
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}

func migrateRawProject(rows *sql.Rows) ([]any, error) {
	var id, source, externalID, rawData, collectedAtStr, hash string
	if err := rows.Scan(&id, &source, &externalID, &rawData, &collectedAtStr, &hash); err != nil {
		return nil, err
	}
	collectedAt, err := parseTime(collectedAtStr)
	if err != nil {
		return nil, err
	}
	return []any{id, source, externalID, rawData, collectedAt, hash}, nil
}

func migrateLead(rows *sql.Rows) ([]any, error) {
	// id, raw_project_id, source, title, location, project_value, general_contractor, project_type,
	// estimated_crew_size, estimated_duration_months, out_of_town_crew_likely, priority_score,
	// priority_reason, suggested_outreach_timing, applicant, contractor, source_url, notes, status,
	// created_at, updated_at
	var (
		id, rawProjectID, source, title, location, generalContractor, projectType,
		priorityReason, suggestedOutreachTiming, applicant, contractor, sourceURL, notes, status,
		createdAtStr, updatedAtStr sql.NullString
		projectValue, estimatedCrewSize, estimatedDurationMonths, outOfTownCrewLikelyInt, priorityScore sql.NullInt64
	)

	if err := rows.Scan(&id, &rawProjectID, &source, &title, &location, &projectValue, &generalContractor, &projectType,
		&estimatedCrewSize, &estimatedDurationMonths, &outOfTownCrewLikelyInt, &priorityScore,
		&priorityReason, &suggestedOutreachTiming, &applicant, &contractor, &sourceURL, &notes, &status,
		&createdAtStr, &updatedAtStr); err != nil {
		return nil, err
	}

	outOfTownCrewLikely := false
	if outOfTownCrewLikelyInt.Valid && outOfTownCrewLikelyInt.Int64 == 1 {
		outOfTownCrewLikely = true
	}

	createdAt, _ := parseTime(createdAtStr.String)
	updatedAt, _ := parseTime(updatedAtStr.String)

	return []any{id, rawProjectID, source, title, location, projectValue, generalContractor, projectType,
		estimatedCrewSize, estimatedDurationMonths, outOfTownCrewLikely, priorityScore,
		priorityReason, suggestedOutreachTiming, applicant, contractor, sourceURL, notes, status,
		createdAt, updatedAt}, nil
}

func migrateOutreachLog(rows *sql.Rows) ([]any, error) {
	var id, leadID, contact, channel, notes, outcome, loggedAtStr sql.NullString
	if err := rows.Scan(&id, &leadID, &contact, &channel, &notes, &outcome, &loggedAtStr); err != nil {
		return nil, err
	}
	loggedAt, _ := parseTime(loggedAtStr.String)
	return []any{id, leadID, contact, channel, notes, outcome, loggedAt}, nil
}
