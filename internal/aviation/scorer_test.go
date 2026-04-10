package aviation

import (
	"testing"

	"github.com/alvindcastro/groupscout/internal/weather"
)

func TestComputeSPS(t *testing.T) {
	tests := []struct {
		name          string
		input         SPSInput
		expectedState AlertState
		minScore      float64
		maxScore      float64
	}{
		{
			name: "Ignore band - no cancellations",
			input: SPSInput{
				CancellationRate: 0.0,
				HourOfDay:        12,
				MinutesActive:    60,
			},
			expectedState: Ignore,
			maxScore:      20,
		},
		{
			name: "Watch band - 15% cancel, daytime, 2h active",
			input: SPSInput{
				CancellationRate: 0.15,
				HourOfDay:        12,
				MinutesActive:    120,
			},
			expectedState: Watch,
			minScore:      20,
			maxScore:      60,
		},
		{
			name: "Soft alert band - 60% cancel, evening, 1h active",
			input: SPSInput{
				CancellationRate: 0.60,
				HourOfDay:        20, // 8pm (multiplier 1.2)
				MinutesActive:    60,
			},
			expectedState: SoftAlert,
			minScore:      60,
			maxScore:      120,
		},
		{
			name: "Hard alert band - 71% cancel, night, 90min active",
			input: SPSInput{
				CancellationRate: 0.71,
				HourOfDay:        22, // 10pm (multiplier 1.5)
				MinutesActive:    90,
			},
			expectedState: HardAlert,
			minScore:      120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeSPS(tt.input)
			if result.State != tt.expectedState {
				t.Errorf("expected state %s, got %s (score: %f)", tt.expectedState, result.State, result.Score)
			}
			if tt.minScore > 0 && result.Score < tt.minScore {
				t.Errorf("expected score >= %f, got %f", tt.minScore, result.Score)
			}
			if tt.maxScore > 0 && result.Score > tt.maxScore {
				t.Errorf("expected score <= %f, got %f", tt.maxScore, result.Score)
			}
		})
	}
}

func TestSPS_VancouverTuning(t *testing.T) {
	t.Run("SnowPreAlert", func(t *testing.T) {
		input := SPSInput{
			CancellationRate: 0.0,
			HourOfDay:        12,
			MinutesActive:    10,
			WeatherAlert: &weather.WeatherAlert{
				Type: weather.Snow,
			},
		}
		result := ComputeSPS(input)
		if result.State != PreAlert {
			t.Errorf("expected state PreAlert for snow, got %s", result.State)
		}
	})

	t.Run("FogRapidResolve", func(t *testing.T) {
		// High cancellation but very short duration fog should be ignored
		input := SPSInput{
			CancellationRate: 0.8,
			HourOfDay:        12,
			MinutesActive:    15,
			WeatherAlert: &weather.WeatherAlert{
				Type: weather.Fog,
			},
		}
		result := ComputeSPS(input)
		if result.State != Ignore {
			t.Errorf("expected state Ignore for short fog, got %s (score: %f)", result.State, result.Score)
		}
	})

	t.Run("AtmosphericRiverLowerThreshold", func(t *testing.T) {
		// Normal watch threshold is 20.
		// If score is 15, usually Ignore.
		// But AtmosphericRiver lowers SPSWatch to 40?
		// Wait, prompt says: "lower SPSWatch threshold to 40 (sustained events)"
		// Current thresholds: Ignore < 20, Watch 20-60, Soft 60-120.
		// If "lower SPSWatch threshold to 40", does it mean Watch starts at 40? No, that's higher.
		// Maybe it means the NEXT threshold?
		// "Thresholds: <20 ignore, 20–60 watch, 60–120 soft alert, >120 hard alert"
		// If I lower Watch threshold to 40... it was 20.
		// Ah, maybe the prompt means the "Soft Alert" threshold is lowered?
		// "AtmosphericRiver: lower SPSWatch threshold to 40 (sustained events)"
		// Let's re-read the thresholds.
		// SPSIgnore = 20
		// SPSWatch = 60
		// SPSSoftAlert = 120
		// So Watch is 20-60, Soft is 60-120, Hard is > 120.
		// If we lower SPSWatch threshold to 40, maybe it means Soft Alert starts at 40 instead of 60?

		input := SPSInput{
			CancellationRate: 0.2, // 0.2 * 160 * 0.58 = 18.56
			HourOfDay:        12,
			MinutesActive:    120, // Duration score 2.0
			WeatherAlert: &weather.WeatherAlert{
				Type: weather.AtmosphericRiver,
			},
		}
		// Score = 18.56 * 2.0 = 37.12
		// Normally this is Watch (20-60).
		// If AtmosphericRiver lowers SoftAlert threshold to 40, it's still Watch.
		// Wait, "lower SPSWatch threshold to 40".
		// If it was 60, and now it's 40. Then 37.12 is still below 40.

		// Let's look at weather.go again.
		// params.SPSWatchThreshold = 40.0 (default was 60.0)
		// So it seems 60 is the end of Watch band.

		result := ComputeSPS(input)
		// If score is 45, it should be SoftAlert if threshold is 40.
		input.CancellationRate = 0.25 // 0.25 * 160 * 0.58 = 23.2
		// 23.2 * 2.0 = 46.4
		result = ComputeSPS(input)
		if result.State != SoftAlert {
			t.Errorf("expected state SoftAlert for atmospheric river at score 46.4, got %s", result.State)
		}
	})

	t.Run("SingleRunwayOpsCap", func(t *testing.T) {
		// High score that would normally be HardAlert
		input := SPSInput{
			CancellationRate: 0.8,
			HourOfDay:        22,
			MinutesActive:    90,
			SingleRunwayOps:  true,
		}
		result := ComputeSPS(input)
		if result.State != SoftAlert {
			t.Errorf("expected state SoftAlert for single runway ops cap, got %s (score: %f)", result.State, result.Score)
		}
	})
}
