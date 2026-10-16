package storage

import (
	"context"
	"testing"
)

func TestLoadActivity_SQLiteReturnsZero(t *testing.T) {
	got, err := LoadActivity(context.Background(), nil, "groupscout.db")
	if err != nil || !got.CadenceRunAt.IsZero() || !got.LeadCreatedAt.IsZero() {
		t.Errorf("SQLite: got %+v, %v; want zero times", got, err)
	}
}
