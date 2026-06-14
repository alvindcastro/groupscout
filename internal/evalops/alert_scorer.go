package evalops

import (
	"fmt"
	"strings"
)

// AlertDecision represents the expected alert decision from a case.
type AlertDecision string

const (
	AlertIgnore     AlertDecision = "ignore"
	AlertWatch      AlertDecision = "watch"
	AlertSoftAlert  AlertDecision = "soft_alert"
	AlertHardAlert  AlertDecision = "hard_alert"
	AlertFailClosed AlertDecision = "fail_closed"
)

// maxFeedAgeSeconds is the threshold beyond which feed data is considered stale.
// 30 minutes (1800 seconds) is the boundary for fresh vs stale in the eval harness.
const maxFeedAgeSeconds = 1800

// AlertScorerResult holds the outcome of alert threshold quality scoring.
type AlertScorerResult struct {
	CaseID            string
	Decision          AlertDecision
	SPSScore          float64
	IsStale           bool
	Findings          []Finding
	IsCriticalFailure bool
}

// ScoreAlertCase evaluates an alert-type Case against its expected decision
// and alert expectations. Uses the same scoring rules as aviation.ComputeSPS
// but operates on fixture data without importing the aviation package.
func ScoreAlertCase(c Case) AlertScorerResult {
	result := AlertScorerResult{CaseID: c.ID}

	if c.CaseType != "alert" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ALERT_WRONG_CASE_TYPE",
			Message:  fmt.Sprintf("expected alert case, got %q", c.CaseType),
		})
		result.IsCriticalFailure = true
		return result
	}

	raw, err := ParseRawAlert(c)
	if err != nil {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ALERT_PARSE_ERROR",
			Message:  fmt.Sprintf("cannot parse raw alert: %v", err),
		})
		result.IsCriticalFailure = true
		return result
	}

	// Stale data check — must fail closed
	isStale := raw.Signals.FeedAgeSeconds > maxFeedAgeSeconds
	result.IsStale = isStale

	if isStale {
		result.Decision = AlertFailClosed
		if c.Expected.Alert != nil && !c.Expected.Alert.StaleData {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ALERT_STALE_FEED_NOT_EXPECTED",
				Message: fmt.Sprintf("feed is %ds old (> %ds threshold) — must fail closed, but case expects stale_data=false",
					raw.Signals.FeedAgeSeconds, maxFeedAgeSeconds),
			})
			result.IsCriticalFailure = true
		}
		// Stale should match fail_closed decision
		if AlertDecision(c.Expected.Decision) != AlertFailClosed {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "ALERT_STALE_DECISION_MISMATCH",
				Message: fmt.Sprintf("stale feed produces fail_closed but expected decision is %q",
					c.Expected.Decision),
			})
		}
		return result
	}

	// Compute SPS score using fixture signal data
	sps := computeFixtureSPS(raw)
	result.SPSScore = sps

	// Determine alert state from score
	decision := alertDecisionFromSPS(sps, raw)
	result.Decision = decision

	// Check decision match
	expectedDecision := AlertDecision(c.Expected.Decision)
	if decision != expectedDecision {
		severity := c.Expected.SeverityOnMismatch
		if severity == "" {
			severity = "warning"
		}
		result.Findings = append(result.Findings, Finding{
			Severity: severity,
			Code:     "ALERT_DECISION_MISMATCH",
			Message: fmt.Sprintf("expected decision %q but got %q (SPS=%.2f)",
				expectedDecision, decision, sps),
		})
		if severity == "critical" {
			result.IsCriticalFailure = true
		}
	}

	// Check score band
	if sps < c.Expected.ScoreBand.Min || sps > c.Expected.ScoreBand.Max {
		severity := c.Expected.SeverityOnMismatch
		if severity == "" {
			severity = "warning"
		}
		result.Findings = append(result.Findings, Finding{
			Severity: severity,
			Code:     "ALERT_SPS_BAND_MISS",
			Message: fmt.Sprintf("SPS %.2f outside expected band [%.1f, %.1f]",
				sps, c.Expected.ScoreBand.Min, c.Expected.ScoreBand.Max),
		})
		if severity == "critical" {
			result.IsCriticalFailure = true
		}
	}

	// Critical gate: single-signal weather-only must not produce priority alert
	if c.Expected.Alert != nil {
		expAlert := c.Expected.Alert

		// Check false priority alert from weather-only noise
		weatherOnly := raw.Signals.WeatherAlert != nil &&
			raw.Signals.CancellationRate < 0.05 &&
			!raw.Signals.NotamActive &&
			raw.Signals.CancelledCount < 5
		if weatherOnly && (decision == AlertSoftAlert || decision == AlertHardAlert) {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ALERT_FALSE_PRIORITY_WEATHER_ONLY",
				Message: fmt.Sprintf("weather-only noise (cancellation rate %.3f, %d cancelled) reached %q — must stay Ignore or Watch",
					raw.Signals.CancellationRate, raw.Signals.CancelledCount, decision),
			})
			result.IsCriticalFailure = true
		}

		// Check single NOTAM with zero cancellations must not reach hard_alert
		notamOnly := raw.Signals.NotamActive &&
			raw.Signals.CancelledCount == 0 &&
			raw.Signals.WeatherAlert == nil
		if notamOnly && decision == AlertHardAlert {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ALERT_FALSE_PRIORITY_NOTAM_ONLY",
				Message:  "NOTAM-only with zero cancellations must not reach hard_alert",
			})
			result.IsCriticalFailure = true
		}

		// Check required signals are mentioned in the raw alert text
		rawText := strings.ToLower(raw.Text)
		for _, sig := range expAlert.RequiredSignals {
			sigKey := strings.ReplaceAll(strings.ToLower(sig), "_", " ")
			// Allow for common signal name variants
			variants := signalVariants(sig)
			found := false
			for _, v := range variants {
				if strings.Contains(rawText, v) {
					found = true
					break
				}
			}
			if !found {
				result.Findings = append(result.Findings, Finding{
					Severity: "warning",
					Code:     "ALERT_MISSING_REQUIRED_SIGNAL",
					Message:  fmt.Sprintf("required signal %q (variant: %q) not found in raw alert text", sig, sigKey),
				})
			}
		}

		// Single runway: must be capped at soft_alert
		if raw.Signals.SingleRunwayOps && decision == AlertHardAlert {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "ALERT_SINGLE_RUNWAY_CAP_MISSED",
				Message:  "single runway ops must cap at SoftAlert, not HardAlert",
			})
		}
	}

	return result
}

// computeFixtureSPS replicates the SPS calculation from aviation.ComputeSPS
// using fixture signal data, without importing the aviation package.
func computeFixtureSPS(raw *RawAlert) float64 {
	if raw == nil {
		return 0
	}
	s := raw.Signals

	avgSeats := s.AvgSeatsPerFlight
	if avgSeats == 0 {
		avgSeats = 160
	}
	connectingRatio := s.ConnectingPaxRatio
	if connectingRatio == 0 {
		connectingRatio = 0.58
	}

	todMultiplier := getFixtureTODMultiplier(s.HourOfDay)
	durationScore := float64(s.MinutesActive) / 60.0
	if durationScore > 3.0 {
		durationScore = 3.0
	}

	score := s.CancellationRate * float64(avgSeats) * connectingRatio * todMultiplier * durationScore

	// Apply Vancouver tuning
	if s.WeatherAlert != nil {
		switch strings.ToLower(s.WeatherAlert.Type) {
		case "atmosphericriver":
			if score >= 40.0 && score < 60.0 {
				score = 60.0 // elevate to Watch boundary
			}
		case "snow":
			if score < 20.0 {
				score = 20.0 // pre-alert floor
			}
		case "fog":
			if s.MinutesActive < 20 {
				score = 0.0 // short fog events ignored
			}
		}
	}

	return score
}

// alertDecisionFromSPS maps an SPS score to an AlertDecision, applying single-runway cap.
func alertDecisionFromSPS(sps float64, raw *RawAlert) AlertDecision {
	var decision AlertDecision
	switch {
	case sps < 20.0:
		decision = AlertIgnore
	case sps < 60.0:
		decision = AlertWatch
	case sps < 120.0:
		decision = AlertSoftAlert
	default:
		decision = AlertHardAlert
	}

	// Single runway cap: hard_alert becomes soft_alert
	if raw != nil && raw.Signals.SingleRunwayOps && decision == AlertHardAlert {
		decision = AlertSoftAlert
	}

	return decision
}

// getFixtureTODMultiplier returns a time-of-day multiplier for the given hour.
func getFixtureTODMultiplier(hour int) float64 {
	switch {
	case hour >= 21 || hour < 0:
		return 1.5
	case hour >= 18:
		return 1.2
	case hour >= 6:
		return 1.0
	default:
		return 0.8
	}
}

// signalVariants returns recognizable text variants of a required signal name.
func signalVariants(signal string) []string {
	base := strings.ToLower(strings.ReplaceAll(signal, "_", " "))
	variants := []string{base}
	switch signal {
	case "cancellation_rate":
		variants = append(variants, "cancellation", "cancelled", "cancel")
	case "weather_alert":
		variants = append(variants, "weather", "atmospheric", "snow", "fog", "notam")
	case "notam_active":
		variants = append(variants, "notam", "restriction", "ils")
	}
	return variants
}
