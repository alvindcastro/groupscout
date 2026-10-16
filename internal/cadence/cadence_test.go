package cadence

import (
	"testing"
	"time"
)

func vancouver(t *testing.T, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestLastDue(t *testing.T) {
	cases := []struct {
		now, want string
	}{
		// Thursday 2026-10-15: before 18:45 the Tuesday run is the latest due.
		{"2026-10-15 10:00", "2026-10-13 18:00"},
		{"2026-10-15 18:44", "2026-10-13 18:00"},
		{"2026-10-15 18:45", "2026-10-15 18:00"},
		// Friday and Saturday still point at Thursday.
		{"2026-10-17 23:59", "2026-10-15 18:00"},
		// Sunday before the slot points at Thursday; after the grace at Sunday.
		{"2026-10-18 12:00", "2026-10-15 18:00"},
		{"2026-10-18 19:00", "2026-10-18 18:00"},
		// Monday points at Sunday.
		{"2026-10-19 09:00", "2026-10-18 18:00"},
		// DST ends Sunday 2026-11-01: the slot stays at 18:00 local time.
		{"2026-11-01 19:00", "2026-11-01 18:00"},
		// DST starts Sunday 2027-03-14.
		{"2027-03-14 18:50", "2027-03-14 18:00"},
	}
	for _, tc := range cases {
		got, err := Default.LastDue(vancouver(t, tc.now))
		if err != nil {
			t.Fatalf("LastDue(%s): %v", tc.now, err)
		}
		if want := vancouver(t, tc.want); !got.Equal(want) {
			t.Errorf("LastDue(%s) = %s, want %s", tc.now, got.In(want.Location()).Format("Mon 2006-01-02 15:04"), tc.want)
		}
	}
}

func TestOverdue(t *testing.T) {
	cases := []struct {
		name, now, lastRun string
		want               bool
	}{
		{"ran on schedule", "2026-10-15 19:00", "2026-10-15 18:01", false},
		{"inside grace, not yet due", "2026-10-15 18:30", "2026-10-13 18:01", false},
		{"missed Thursday", "2026-10-15 18:45", "2026-10-13 18:01", true},
		{"missed Thursday, still overdue Saturday", "2026-10-17 12:00", "2026-10-13 18:01", true},
		{"late run within grace counts", "2026-10-15 19:00", "2026-10-15 18:40", false},
		{"no run recorded", "2026-10-15 19:00", "", true},
	}
	for _, tc := range cases {
		var last time.Time
		if tc.lastRun != "" {
			last = vancouver(t, tc.lastRun)
		}
		got, err := Default.Overdue(vancouver(t, tc.now), last)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: Overdue = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLastDueErrors(t *testing.T) {
	if _, err := (Schedule{Days: nil, Location: "America/Vancouver"}).LastDue(time.Now()); err == nil {
		t.Error("a schedule with no days must error")
	}
	if _, err := (Schedule{Days: Default.Days, Location: "Nowhere/Invalid"}).LastDue(time.Now()); err == nil {
		t.Error("an unknown location must error")
	}
}
