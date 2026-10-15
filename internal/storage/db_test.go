package storage

import (
	"strings"
	"testing"
)

func TestDriverName(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"groupscout.db", "sqlite"},
		{"./data/groupscout.db", "sqlite"},
		{"postgres://user:pass@localhost:5432/groupscout", "pgx"},
		{"postgresql://user:pass@localhost:5432/groupscout", "pgx"},
		{"postgres://localhost/groupscout?sslmode=disable", "pgx"},
	}
	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			got := DriverName(tt.dsn)
			if got != tt.want {
				t.Errorf("DriverName(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestRebind(t *testing.T) {
	tests := []struct {
		name  string
		dsn   string
		query string
		want  string
	}{
		{
			"sqlite no-op",
			"groupscout.db",
			"INSERT INTO t (a, b) VALUES (?, ?)",
			"INSERT INTO t (a, b) VALUES (?, ?)",
		},
		{
			"postgres rebind",
			"postgres://localhost",
			"INSERT INTO t (a, b) VALUES (?, ?)",
			"INSERT INTO t (a, b) VALUES ($1, $2)",
		},
		{
			"postgres complex",
			"postgres://localhost",
			"UPDATE t SET a = ? WHERE b = ? AND c = ?",
			"UPDATE t SET a = $1 WHERE b = $2 AND c = $3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rebind(tt.dsn, tt.query)
			if got != tt.want {
				t.Errorf("Rebind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactDSN(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"groupscout.db", "groupscout.db"},
		{"postgres://groupscout:s3cret@postgres:5432/groupscout?sslmode=disable", "postgres://groupscout:xxxxx@postgres:5432/groupscout?sslmode=disable"},
		{"postgresql://user:p%40ss%2Fword@localhost/db", "postgresql://user:xxxxx@localhost/db"},
		{"postgres://groupscout@localhost/groupscout", "postgres://groupscout@localhost/groupscout"},
		{"postgres://localhost/db?user=u&password=s3cret", "postgres://localhost/db?password=xxxxx&user=u"},
		{"postgres://u:s3cret@host:bad-port/db", "<unparseable database url>"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RedactDSN(tt.dsn)
			if got != tt.want {
				t.Errorf("RedactDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
			if strings.Contains(got, "s3cret") || strings.Contains(got, "p%40ss") {
				t.Errorf("RedactDSN(%q) leaked the password: %q", tt.dsn, got)
			}
		})
	}
}
