package storage

import "testing"

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
