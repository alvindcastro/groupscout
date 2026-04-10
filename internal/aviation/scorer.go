package aviation

import (
	"fmt"
	"math"

	"github.com/alvindcastro/groupscout/internal/weather"
)

type AlertState string

const (
	Ignore    AlertState = "Ignore"
	Watch     AlertState = "Watch"
	SoftAlert AlertState = "SoftAlert"
	HardAlert AlertState = "HardAlert"
	PreAlert  AlertState = "PreAlert"
)

const (
	SPSIgnore    = 20.0
	SPSWatch     = 60.0
	SPSSoftAlert = 120.0
)

type SPSInput struct {
	CancellationRate   float64
	AvgSeatsPerFlight  int     // default 160
	ConnectingPaxRatio float64 // default 0.58 for YVR
	HourOfDay          int     // 0–23
	MinutesActive      int
	WeatherAlert       *weather.WeatherAlert // nil if no active alert
	SingleRunwayOps    bool                  // if true, cap at SoftAlert
}

type SPSResult struct {
	Score       float64
	State       AlertState
	Explanation string
}

func ComputeSPS(input SPSInput) SPSResult {
	avgSeats := input.AvgSeatsPerFlight
	if avgSeats == 0 {
		avgSeats = 160
	}

	connectingRatio := input.ConnectingPaxRatio
	if connectingRatio == 0 {
		connectingRatio = 0.58
	}

	todMultiplier := getTimeOfDayMultiplier(input.HourOfDay)
	durationScore := math.Min(float64(input.MinutesActive)/60.0, 3.0)

	score := input.CancellationRate * float64(avgSeats) * connectingRatio * todMultiplier * durationScore

	result := SPSResult{
		Score: score,
	}

	result.State = AlertStateFromScore(score)
	result.Explanation = fmt.Sprintf("Score %.2f (Rate: %.2f, TOD: %.1f, Duration: %.2f)",
		score, input.CancellationRate, todMultiplier, durationScore)

	ApplyVancouverTuning(&result, input.WeatherAlert, input.MinutesActive, input.SingleRunwayOps)

	return result
}

func AlertStateFromScore(score float64) AlertState {
	if score < SPSIgnore {
		return Ignore
	}
	if score < SPSWatch {
		return Watch
	}
	if score < SPSSoftAlert {
		return SoftAlert
	}
	return HardAlert
}

func ApplyVancouverTuning(result *SPSResult, alert *weather.WeatherAlert, minutesActive int, singleRunway bool) {
	if singleRunway && result.State == HardAlert {
		result.State = SoftAlert
		result.Explanation += " [Single Runway tuning: cap at soft alert]"
	}

	if alert == nil {
		return
	}

	switch alert.Type {
	case weather.Fog:
		if minutesActive < 20 {
			result.State = Ignore
			result.Explanation += " [Fog tuning: short duration ignore]"
		}
	case weather.Snow:
		if result.State == Ignore {
			result.State = PreAlert
			result.Explanation += " [Snow tuning: pre-alert active]"
		}
	case weather.AtmosphericRiver:
		// lower SPSWatch threshold to 40 (sustained events)
		// Normally Watch is 20-60. With tuning, Watch is 20-40, Soft is 40-120.
		if result.Score >= 40.0 && result.State == Watch {
			result.State = SoftAlert
			result.Explanation += " [Atmospheric River tuning: lower soft alert threshold]"
		}
	}
}

func getTimeOfDayMultiplier(hour int) float64 {
	switch {
	case hour >= 21 || hour < 0: // 9pm–midnight
		return 1.5
	case hour >= 18: // 6pm–9pm
		return 1.2
	case hour >= 6: // 6am–6pm
		return 1.0
	default: // midnight–6am
		return 0.8
	}
}
