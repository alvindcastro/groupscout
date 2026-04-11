package ollama

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAlertCopyGenerator_Generate(t *testing.T) {
	fixtureData, err := os.ReadFile("testdata/disruption.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var fixtureEvent DisruptionEvent
	if err := json.Unmarshal(fixtureData, &fixtureEvent); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	tests := []struct {
		name         string
		event        DisruptionEvent
		mockResponse string
		mockErr      error
		wantContains []string
		wantErr      bool
	}{
		{
			name:         "Atmospheric river hard alert",
			event:        fixtureEvent,
			mockResponse: "- Atmospheric river causing high cancellations.\n- Pull back OTA inventory immediately.\n- Expect recovery after 2am.",
			wantContains: []string{"-", "Atmospheric river", "inventory"},
		},
		{
			name: "Fog event",
			event: DisruptionEvent{
				Cause:            "fog",
				SPS:              4.2,
				AlertState:       "soft_alert",
				FlightsCancelled: 10,
				FlightsScheduled: 120,
				EstStranded:      150,
				StartedAt:        time.Now().Add(-1 * time.Hour),
			},
			mockResponse: "- Dense fog impacting arrivals.\n- Activate distressed rate for stranded pax.\n- Monitor NavCanada for updates.",
			wantContains: []string{"-", "fog", "distressed rate"},
		},
		{
			name: "Resolve state",
			event: DisruptionEvent{
				Cause:      "snow",
				AlertState: "resolve",
			},
			mockResponse: "Weather has cleared. Operations returning to normal. Alert resolved.",
			wantContains: []string{"cleared", "resolved"},
		},
		{
			name:    "Context cancellation",
			event:   fixtureEvent,
			mockErr: context.Canceled,
			wantErr: false,
		},
		{
			name:    "Ollama unavailable",
			event:   fixtureEvent,
			mockErr: os.ErrNotExist,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLLMClient{
				chatCompleteFunc: func(ctx context.Context, system, user string) (string, error) {
					return tt.mockResponse, tt.mockErr
				},
			}
			g := NewAlertCopyGenerator(mock)
			got, err := g.Generate(context.Background(), tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got == "" {
					if tt.mockErr == nil && tt.mockResponse != "" {
						t.Error("Generate() returned empty string unexpectedly")
					}
					return
				}
				for _, want := range tt.wantContains {
					if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
						t.Errorf("Generate() output = %v, want to contain %v", got, want)
					}
				}
			}
		})
	}
}
