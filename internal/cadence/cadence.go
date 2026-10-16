// Package cadence describes when the lead cadence is due, so monitoring can
// tell a missed run from a quiet one (IaC Phase 9, groupscout-site-6ah.10).
//
// The n8n workflow (deploy/config/n8n/workflows/sunday-wednesday-lead-cadence.json)
// triggers the run and the Healthchecks.io check
// (deploy/config/healthchecks/checks.json) watches for it; internal/smoke
// checks that both agree with Default.
package cadence

import (
	"fmt"
	"slices"
	"time"

	// The Alpine runtime image has no /usr/share/zoneinfo; embed the zone
	// database so America/Vancouver resolves inside the container.
	_ "time/tzdata"
)

// Schedule is a weekly run time in a named time zone. Grace is how long after
// the scheduled time a run may finish before it counts as missed.
type Schedule struct {
	Days     []time.Weekday
	Hour     int
	Minute   int
	Location string
	Grace    time.Duration
}

// Default is the production lead cadence: Sunday, Tuesday and Thursday at
// 18:00 Vancouver time. A run takes about a minute; the grace leaves room for
// n8n's retries and a slow collector.
var Default = Schedule{
	Days:     []time.Weekday{time.Sunday, time.Tuesday, time.Thursday},
	Hour:     18,
	Minute:   0,
	Location: "America/Vancouver",
	Grace:    45 * time.Minute,
}

// LastDue returns the most recent scheduled run time whose grace period had
// ended by now: the latest run a healthy cadence must have completed.
func (s Schedule) LastDue(now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(s.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("cadence: load location %q: %w", s.Location, err)
	}
	local := now.In(loc)
	// Eight days back always reaches the previous occurrence of every weekday,
	// even when today's slot is still inside its grace period.
	for back := 0; back <= 8; back++ {
		day := local.AddDate(0, 0, -back)
		if !slices.Contains(s.Days, day.Weekday()) {
			continue
		}
		slot := time.Date(day.Year(), day.Month(), day.Day(), s.Hour, s.Minute, 0, 0, loc)
		if !slot.Add(s.Grace).After(now) {
			return slot, nil
		}
	}
	return time.Time{}, fmt.Errorf("cadence: schedule has no run days")
}

// Overdue reports whether the run due at LastDue(now) never completed, given
// when the last cadence run completed.
func (s Schedule) Overdue(now, lastRun time.Time) (bool, error) {
	due, err := s.LastDue(now)
	if err != nil {
		return false, err
	}
	return lastRun.Before(due), nil
}
