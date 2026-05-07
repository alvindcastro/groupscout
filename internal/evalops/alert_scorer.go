package evalops

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ScoreAlertThreshold(c Case, slackText string) Result {
	const scorer = "alert_threshold"
	if c.CaseType != CaseTypeAlert {
		return failResult(c, scorer, SeverityWarning, false, "", 0, "case is not an alert")
	}
	decision, score, degraded, message := deriveAlertDecision(c.Raw.Signals)
	if degraded {
		return failResult(c, scorer, SeverityWarning, false, decision, score, message)
	}
	if decision != c.Expected.Decision {
		return mismatchResult(c, scorer, decision, score, fmt.Sprintf("alert decision %s outside expected %s", decision, c.Expected.Decision))
	}
	if !scoreInBand(score, c.Expected.ScoreBand) {
		return mismatchResult(c, scorer, decision, score, fmt.Sprintf("alert score %d outside expected band %d-%d", score, c.Expected.ScoreBand.Min, c.Expected.ScoreBand.Max))
	}
	if decision == "hard_alert" && !prioritySlackIncludesSignals(c.Raw.Signals, slackText) {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, decision, score, "priority Slack alert text must include source signals")
	}
	return passResult(c, scorer, decision, score, "alert threshold satisfies fixture expectations")
}

func deriveAlertDecision(signals AlertSignals) (decision string, score int, degraded bool, message string) {
	if signals.FeedAgeMinutes > 30 {
		return "fail_closed", 0, false, "stale feed fails closed"
	}
	if signals.Notams == nil {
		score = computeAlertScore(signals)
		return alertDecisionFromScore(score), score, true, "missing NOTAM feed marks alert evaluation degraded"
	}
	score = computeAlertScore(signals)
	decision = alertDecisionFromScore(score)
	if decision == "hard_alert" && corroboratingAlertSignals(signals) < 2 {
		decision = "soft_alert"
		score = min(score, 119)
	}
	return decision, score, false, ""
}

func computeAlertScore(signals AlertSignals) int {
	score := signals.CancellationRate * 300
	if signals.CancelledCount >= 30 {
		score += 10
	}
	if signals.WeatherAlert != nil {
		switch strings.ToLower(signals.WeatherAlert.Severity) {
		case "warning", "severe":
			score += 25
		default:
			score += 5
		}
	}
	if signals.Notams != nil {
		for _, notam := range *signals.Notams {
			switch strings.ToLower(notam.CapacityImpact) {
			case "major":
				score += 25
			case "minor":
				score += 5
			}
		}
	}
	if signals.MinutesActive >= 120 {
		score += 15
	}
	if signals.HourOfDay >= 18 || signals.HourOfDay < 6 {
		score += 10
	}
	if signals.SingleRunwayOps && score >= 120 {
		score = 119
	}
	return int(math.Round(score))
}

func alertDecisionFromScore(score int) string {
	switch {
	case score >= 120:
		return "hard_alert"
	case score >= 60:
		return "soft_alert"
	case score >= 20:
		return "watch"
	default:
		return "ignore"
	}
}

func corroboratingAlertSignals(signals AlertSignals) int {
	var count int
	if signals.CancellationRate >= 0.15 || signals.CancelledCount >= 30 {
		count++
	}
	if signals.WeatherAlert != nil && strings.EqualFold(signals.WeatherAlert.Severity, "warning") {
		count++
	}
	if signals.Notams != nil {
		for _, notam := range *signals.Notams {
			if strings.EqualFold(notam.CapacityImpact, "major") {
				count++
				break
			}
		}
	}
	return count
}

func prioritySlackIncludesSignals(signals AlertSignals, slackText string) bool {
	lower := strings.ToLower(slackText)
	if !strings.Contains(lower, strconv.Itoa(signals.CancelledCount)) {
		return false
	}
	if signals.WeatherAlert != nil {
		weatherType := strings.Split(strings.ReplaceAll(strings.ToLower(signals.WeatherAlert.Type), "_", " "), " ")[0]
		if weatherType != "" && !strings.Contains(lower, weatherType) {
			return false
		}
	}
	if signals.Notams != nil && len(*signals.Notams) > 0 {
		if !strings.Contains(lower, "notam") && !strings.Contains(lower, "capacity") && !strings.Contains(lower, "arrival rate") {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
