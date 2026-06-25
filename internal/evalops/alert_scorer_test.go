package evalops

import (
	"encoding/json"
	"os"
	"testing"
)

func makeAlertCase(id, decision, riskLevel string, signals AlertSignals, scoreBandMin, scoreBandMax float64) Case {
	raw := RawAlert{
		AirportCode: "YVR",
		ObservedAt:  "2026-06-10T19:30:00Z",
		Text:        buildAlertText(signals),
		Signals:     signals,
	}
	rawBytes, _ := json.Marshal(raw)

	expAlert := &ExpectedAlert{
		State:           decision,
		Priority:        "high",
		StaleData:       signals.FeedAgeSeconds > maxFeedAgeSeconds,
		RequiredSignals: []string{},
	}

	return Case{
		ID:        id,
		CaseType:  "alert",
		Category:  "airport_disruption",
		RiskLevel: riskLevel,
		Source: CaseSource{
			System:      "yvr_disruption",
			Type:        "multi_signal_snapshot",
			FixtureURL:  "https://fixtures.groupscout.local/yvr/test.json",
			CollectedAt: "2026-06-10T19:30:00Z",
		},
		Raw: json.RawMessage(rawBytes),
		Expected: CaseExpected{
			EvalResult:         "pass",
			Decision:           decision,
			ReleaseBlocking:    riskLevel == "critical",
			SeverityOnMismatch: riskLevel,
			ScoreBand:          ScoreBand{Min: scoreBandMin, Max: scoreBandMax},
			Alert:              expAlert,
			Privacy:            PrivacyExpectation{},
		},
	}
}

func buildAlertText(s AlertSignals) string {
	text := "YVR alert snapshot."
	if s.CancelledCount > 0 {
		text += " cancellation spike detected."
	}
	if s.WeatherAlert != nil {
		text += " weather alert active: " + s.WeatherAlert.Type + "."
	}
	if s.NotamActive {
		text += " NOTAM active."
	}
	if s.SingleRunwayOps {
		text += " single runway ops."
	}
	return text
}

func multiSignalSevereDisruption() AlertSignals {
	// 50% cancellation rate over 3 hours at evening peak produces SPS ~167 (HardAlert)
	return AlertSignals{
		CancelledCount:     70,
		TotalFlights:       140,
		CancellationRate:   0.5,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          19,
		MinutesActive:      180,
		WeatherAlert:       &WeatherSignal{Type: "AtmosphericRiver", Severity: "HIGH"},
		NotamActive:        true,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     120,
	}
}

func TestScoreAlertCase_MultiSignalSevereDisruptionTriggersPriority(t *testing.T) {
	signals := multiSignalSevereDisruption()
	c := makeAlertCase("alert-multi-001", "hard_alert", "critical", signals, 120, 600)

	result := ScoreAlertCase(c)

	if result.Decision != AlertHardAlert && result.Decision != AlertSoftAlert {
		t.Errorf("expected HardAlert or SoftAlert for multi-signal severe disruption, got %q (SPS=%.2f)", result.Decision, result.SPSScore)
	}
	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure for valid multi-signal case, findings: %+v", result.Findings)
	}
}

func TestScoreAlertCase_WeatherOnlyNoiseDoesNotTriggerPriority(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     2,
		TotalFlights:       160,
		CancellationRate:   0.0125,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          14,
		MinutesActive:      8,
		WeatherAlert:       &WeatherSignal{Type: "Fog", Severity: "ADVISORY"},
		NotamActive:        false,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     200,
	}
	c := makeAlertCase("alert-weather-only-001", "ignore", "critical", signals, 0, 20)

	result := ScoreAlertCase(c)

	// Fog with very short duration should result in SPS=0 (fog tuning ignores <20 min)
	if result.Decision == AlertSoftAlert || result.Decision == AlertHardAlert {
		t.Errorf("weather-only noise must not reach SoftAlert/HardAlert, got %q (SPS=%.2f)", result.Decision, result.SPSScore)
	}
	if result.IsCriticalFailure {
		t.Errorf("unexpected critical failure for weather-only noise case, findings: %+v", result.Findings)
	}
}

func TestScoreAlertCase_StaleDataFailsClosed(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     20,
		TotalFlights:       120,
		CancellationRate:   0.167,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          22,
		MinutesActive:      40,
		WeatherAlert:       &WeatherSignal{Type: "Snow", Severity: "MODERATE"},
		NotamActive:        false,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     2700, // 45 minutes — stale
	}
	c := makeAlertCase("alert-stale-001", "fail_closed", "critical", signals, 0, 0)
	c.Expected.Alert = &ExpectedAlert{
		State:     "Degraded",
		Priority:  "none",
		StaleData: true,
	}

	result := ScoreAlertCase(c)

	if result.Decision != AlertFailClosed {
		t.Errorf("expected fail_closed for stale feed, got %q", result.Decision)
	}
	if !result.IsStale {
		t.Error("expected IsStale=true for feed age > threshold")
	}
	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure when case correctly expects fail_closed, findings: %+v", result.Findings)
	}
}

func TestScoreAlertCase_StaleDataPriorityAlertIsCritical(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     20,
		TotalFlights:       120,
		CancellationRate:   0.167,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          22,
		MinutesActive:      40,
		WeatherAlert:       nil,
		NotamActive:        false,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     3600, // 1 hour stale
	}
	// Case expects hard_alert (wrong — stale feed should fail closed)
	c := makeAlertCase("alert-stale-wrong-001", "hard_alert", "critical", signals, 60, 200)
	c.Expected.Alert = &ExpectedAlert{
		State:     "HardAlert",
		Priority:  "high",
		StaleData: false, // case claims data is fresh, but signals say it's stale
	}

	result := ScoreAlertCase(c)

	// Scorer detects stale, so decision is fail_closed
	if result.Decision != AlertFailClosed {
		t.Errorf("expected fail_closed for stale feed, got %q", result.Decision)
	}
	// Since expected.Alert.StaleData=false but data IS stale, we get a critical finding
	if !result.IsCriticalFailure {
		t.Error("expected critical failure when stale feed expected stale_data=false")
	}
}

func TestScoreAlertCase_NotamOnlyDoesNotHardAlert(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     0,
		TotalFlights:       95,
		CancellationRate:   0.0,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          21,
		MinutesActive:      5,
		WeatherAlert:       nil,
		NotamActive:        true,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     90,
	}
	c := makeAlertCase("alert-notam-001", "watch", "warning", signals, 0, 30)
	c.Expected.Alert = &ExpectedAlert{
		State:           "Watch",
		Priority:        "none",
		StaleData:       false,
		RequiredSignals: []string{"notam_active"},
	}

	result := ScoreAlertCase(c)

	if result.Decision == AlertHardAlert {
		t.Errorf("NOTAM-only with zero cancellations must not reach hard_alert, got %q (SPS=%.2f)", result.Decision, result.SPSScore)
	}
	// No critical failure expected for correctly handled case
	if result.IsCriticalFailure {
		t.Errorf("unexpected critical failure for NOTAM-only case, findings: %+v", result.Findings)
	}
}

func TestScoreAlertCase_SnowSingleRunwayCapSoftAlert(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     5,
		TotalFlights:       85,
		CancellationRate:   0.059,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          7,
		MinutesActive:      25,
		WeatherAlert:       &WeatherSignal{Type: "Snow", Severity: "MODERATE"},
		NotamActive:        false,
		SingleRunwayOps:    true,
		FeedAgeSeconds:     60,
	}
	c := makeAlertCase("alert-snow-single-runway-001", "soft_alert", "warning", signals, 15, 120)

	result := ScoreAlertCase(c)

	// Decision should not be hard_alert due to single-runway cap
	if result.Decision == AlertHardAlert {
		t.Errorf("single runway ops must cap at SoftAlert, got %q (SPS=%.2f)", result.Decision, result.SPSScore)
	}
	if result.IsCriticalFailure {
		t.Errorf("unexpected critical failure for snow/single-runway case, findings: %+v", result.Findings)
	}
}

func TestScoreAlertCase_ThresholdBoundaryBehavior(t *testing.T) {
	tests := []struct {
		name             string
		signals          AlertSignals
		expectedDecision AlertDecision
	}{
		{
			name: "just below Watch threshold (SPS ~19)",
			signals: AlertSignals{
				CancellationRate:   0.05,
				AvgSeatsPerFlight:  160,
				ConnectingPaxRatio: 0.58,
				HourOfDay:          12,
				MinutesActive:      24, // durationScore=0.4
				FeedAgeSeconds:     60,
			},
			expectedDecision: AlertIgnore,
		},
		{
			// rate=0.25, seats=160, ratio=0.58, tod=1.0, dur=1.5 -> SPS = 0.25*160*0.58*1.0*1.5 = 34.8 -> Watch
			name: "well above Ignore but below Watch",
			signals: AlertSignals{
				CancellationRate:   0.25,
				TotalFlights:       100,
				CancelledCount:     25,
				AvgSeatsPerFlight:  160,
				ConnectingPaxRatio: 0.58,
				HourOfDay:          12,
				MinutesActive:      90,
				FeedAgeSeconds:     60,
			},
			expectedDecision: AlertWatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sps := computeFixtureSPS(&RawAlert{Signals: tt.signals})
			actual := alertDecisionFromSPS(sps, &RawAlert{Signals: tt.signals})
			if actual != tt.expectedDecision {
				t.Errorf("%s: expected %q, got %q (SPS=%.2f)", tt.name, tt.expectedDecision, actual, sps)
			}
		})
	}
}

func TestScoreAlertCase_ParseError(t *testing.T) {
	c := Case{
		ID:       "alert-malformed-001",
		CaseType: "alert",
		Raw:      json.RawMessage(`{invalid`),
		Expected: CaseExpected{Decision: "fail_closed"},
	}

	result := ScoreAlertCase(c)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for malformed alert raw")
	}
	if !hasFindingCode(result.Findings, "ALERT_PARSE_ERROR") {
		t.Errorf("expected ALERT_PARSE_ERROR finding, got: %+v", result.Findings)
	}
}

func TestScoreAlertCase_WrongCaseType(t *testing.T) {
	c := Case{
		ID:       "lead-wrong-type",
		CaseType: "lead",
		Expected: CaseExpected{Decision: "keep"},
	}

	result := ScoreAlertCase(c)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for wrong case type")
	}
}

func TestScoreAlertCase_FixtureCases(t *testing.T) {
	fixtureDir := "../../data/evals/groupscout"
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skip("fixture dir not found")
	}

	cases, err := LoadCasesFromDir(fixtureDir)
	if err != nil {
		t.Fatalf("load fixture cases: %v", err)
	}

	for _, c := range cases {
		if c.CaseType != "alert" {
			continue
		}
		result := ScoreAlertCase(c)
		if result.IsCriticalFailure && c.Expected.ReleaseBlocking {
			t.Errorf("fixture alert case %q: critical failure — %s",
				c.ID, fixturesFindingsStr(result.Findings))
		}
	}
}
