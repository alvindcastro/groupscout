package evalops

import (
	"strings"
	"testing"
)

func TestScoreAlertThresholdPassesMultiSignalSeverePriority(t *testing.T) {
	c := alertCase("alert-hard", AlertSignals{
		CancelledCount:     72,
		TotalFlights:       260,
		CancellationRate:   0.277,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          22,
		MinutesActive:      130,
		WeatherAlert:       &WeatherSignal{Type: "atmospheric_river", Severity: "warning"},
		Notams:             &[]NotamSignal{{Type: "arrival_rate_reduction", Impact: "reduced_capacity", CapacityImpact: "major"}},
		FeedAgeMinutes:     3,
	}, "hard_alert", ScoreBand{Min: 120, Max: 999})
	slackText := "YVR: 72 cancellations, atmospheric river warning, arrival rate reduction NOTAM."

	result := ScoreAlertThreshold(c, slackText)
	if result.Status != ResultPass {
		t.Fatalf("ScoreAlertThreshold() = %+v, want pass", result)
	}
	if result.Decision != "hard_alert" || result.Score < 120 {
		t.Fatalf("decision/score = %s/%d, want hard_alert >= 120", result.Decision, result.Score)
	}
}

func TestScoreAlertThresholdDoesNotPrioritizeWeatherOnlyNoise(t *testing.T) {
	c := alertCase("alert-weather-only", AlertSignals{
		CancelledCount:     2,
		TotalFlights:       238,
		CancellationRate:   0.008,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          20,
		MinutesActive:      15,
		WeatherAlert:       &WeatherSignal{Type: "fog", Severity: "advisory"},
		Notams:             &[]NotamSignal{},
		FeedAgeMinutes:     5,
	}, "ignore", ScoreBand{Min: 0, Max: 19})

	result := ScoreAlertThreshold(c, "")
	if result.Status != ResultPass || result.Decision != "ignore" {
		t.Fatalf("ScoreAlertThreshold() = %+v, want ignore pass", result)
	}
}

func TestScoreAlertThresholdFailsClosedOnStaleData(t *testing.T) {
	c := alertCase("alert-stale", AlertSignals{
		CancelledCount:     65,
		TotalFlights:       250,
		CancellationRate:   0.26,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          19,
		MinutesActive:      90,
		WeatherAlert:       &WeatherSignal{Type: "snow", Severity: "warning"},
		Notams:             &[]NotamSignal{{Type: "runway_condition", CapacityImpact: "major"}},
		SingleRunwayOps:    true,
		FeedAgeMinutes:     410,
	}, "fail_closed", ScoreBand{Min: 0, Max: 0})

	result := ScoreAlertThreshold(c, "")
	if result.Status != ResultPass || result.Decision != "fail_closed" || result.Score != 0 {
		t.Fatalf("ScoreAlertThreshold() = %+v, want fail_closed pass", result)
	}
}

func TestScoreAlertThresholdMarksMissingNotamFeedDegradedWarning(t *testing.T) {
	c := alertCase("alert-missing-notam", AlertSignals{
		CancelledCount:     30,
		TotalFlights:       240,
		CancellationRate:   0.125,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          19,
		MinutesActive:      60,
		WeatherAlert:       &WeatherSignal{Type: "snow", Severity: "warning"},
		Notams:             nil,
		FeedAgeMinutes:     6,
	}, "soft_alert", ScoreBand{Min: 60, Max: 119})

	result := ScoreAlertThreshold(c, "")
	if result.Status != ResultFail || result.Severity != SeverityWarning {
		t.Fatalf("ScoreAlertThreshold() = %+v, want warning degraded missing NOTAM feed", result)
	}
	if !strings.Contains(result.Message, "NOTAM") {
		t.Fatalf("ScoreAlertThreshold() message = %q, want NOTAM degraded warning", result.Message)
	}
}

func TestScoreAlertThresholdBoundaryBehavior(t *testing.T) {
	ignore := alertCase("alert-boundary-ignore", AlertSignals{
		CancelledCount:   3,
		TotalFlights:     200,
		CancellationRate: 0.015,
		MinutesActive:    60,
		Notams:           &[]NotamSignal{},
		FeedAgeMinutes:   5,
	}, "ignore", ScoreBand{Min: 0, Max: 19})
	watch := alertCase("alert-boundary-watch", AlertSignals{
		CancelledCount:   20,
		TotalFlights:     200,
		CancellationRate: 0.10,
		MinutesActive:    60,
		Notams:           &[]NotamSignal{},
		FeedAgeMinutes:   5,
	}, "watch", ScoreBand{Min: 20, Max: 59})

	if result := ScoreAlertThreshold(ignore, ""); result.Status != ResultPass || result.Decision != "ignore" {
		t.Fatalf("ignore boundary result = %+v, want ignore pass", result)
	}
	if result := ScoreAlertThreshold(watch, ""); result.Status != ResultPass || result.Decision != "watch" {
		t.Fatalf("watch boundary result = %+v, want watch pass", result)
	}
}

func TestScoreAlertThresholdRequiresSlackSourceSignalsForPriority(t *testing.T) {
	c := alertCase("alert-hard-missing-slack-signals", AlertSignals{
		CancelledCount:     72,
		TotalFlights:       260,
		CancellationRate:   0.277,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          22,
		MinutesActive:      130,
		WeatherAlert:       &WeatherSignal{Type: "atmospheric_river", Severity: "warning"},
		Notams:             &[]NotamSignal{{Type: "arrival_rate_reduction", Impact: "reduced_capacity", CapacityImpact: "major"}},
		FeedAgeMinutes:     3,
	}, "hard_alert", ScoreBand{Min: 120, Max: 999})

	result := ScoreAlertThreshold(c, "YVR priority disruption.")
	if result.Status != ResultFail || result.Severity != SeverityCritical {
		t.Fatalf("ScoreAlertThreshold() = %+v, want critical missing Slack signals", result)
	}
}

func alertCase(id string, signals AlertSignals, decision string, band ScoreBand) Case {
	return Case{
		ID:        id,
		CaseType:  CaseTypeAlert,
		Category:  "airport_disruption",
		RiskLevel: SeverityCritical,
		Source:    Source{System: "yvr_disruption", Type: "multi_signal_snapshot", FixtureURL: "https://fixtures.groupscout.local/" + id, CollectedAt: "2026-05-07T00:00:00Z"},
		Raw: RawPayload{
			AirportCode: "YVR",
			ObservedAt:  "2026-05-07T05:20:00Z",
			Text:        "Synthetic YVR disruption snapshot.",
			Signals:     signals,
		},
		Expected: ExpectedOutcome{
			EvalResult:         "pass",
			Decision:           decision,
			ReleaseBlocking:    true,
			SeverityOnMismatch: SeverityCritical,
			ScoreBand:          band,
			Alert:              &AlertExpectation{State: "derived", Priority: decision == "hard_alert", StaleData: decision == "fail_closed"},
			Evidence:           []EvidenceRequirement{{Claim: "source signals", MustSupportWith: "Synthetic YVR"}},
			Privacy:            PrivacyExpectation{},
		},
	}
}
