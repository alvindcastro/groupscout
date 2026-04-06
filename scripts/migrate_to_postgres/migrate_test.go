package main

import "testing"

func TestParseArgs_defaults(t *testing.T) {
	args, err := parseArgs([]string{
		"--sqlite", "groupscout.db",
		"--postgres", "postgres://localhost/groupscout",
	})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if args.sqliteDSN != "groupscout.db" {
		t.Errorf("sqliteDSN = %q, want groupscout.db", args.sqliteDSN)
	}
	if args.postgresDSN != "postgres://localhost/groupscout" {
		t.Errorf("postgresDSN wrong")
	}
	if args.dryRun != false {
		t.Error("dryRun should default to false")
	}
}

func TestParseArgs_dry_run(t *testing.T) {
	args, _ := parseArgs([]string{
		"--sqlite", "x.db",
		"--postgres", "postgres://localhost/x",
		"--dry-run",
	})
	if !args.dryRun {
		t.Error("dryRun should be true when flag set")
	}
}

func TestParseArgs_missing_required(t *testing.T) {
	_, err := parseArgs([]string{"--sqlite", "x.db"})
	if err == nil {
		t.Error("expected error when --postgres is missing")
	}
}
