package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseWeatherAlerts(t *testing.T) {
	fixture := `{
		"type": "FeatureCollection",
		"features": [
			{
				"properties": {
					"event": "SPECIAL WEATHER STATEMENT",
					"headline": "Atmospheric river to bring heavy rain",
					"severity": "Moderate",
					"zone": "BC_14_09"
				}
			},
			{
				"properties": {
					"event": "FREEZING FOG ADVISORY",
					"headline": "Patchy freezing fog expected",
					"severity": "Minor",
					"zone": "BC_14_09"
				}
			}
		]
	}`

	alerts, err := parseWeatherAlerts([]byte(fixture))
	if err != nil {
		t.Fatalf("failed to parse alerts: %v", err)
	}

	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}

	if alerts[0].Type != AtmosphericRiver {
		t.Errorf("expected AtmosphericRiver, got %v", alerts[0].Type)
	}
	if alerts[1].Type != Fog {
		t.Errorf("expected Fog, got %v", alerts[1].Type)
	}
}

func TestClassifyAlertType(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		headline string
		expected AlertType
	}{
		{"Snow Warning", "SNOWFALL WARNING", "", Snow},
		{"Freezing Fog", "FREEZING FOG ADVISORY", "", Fog},
		{"Atmospheric River", "SPECIAL WEATHER STATEMENT", "An atmospheric river is coming", AtmosphericRiver},
		{"Wind Warning", "WIND WARNING", "", Wind},
		{"Unknown", "HEAT WARNING", "", Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyAlertType(tt.event, tt.headline)
			if got != tt.expected {
				t.Errorf("ClassifyAlertType(%q, %q) = %v; want %v", tt.event, tt.headline, got, tt.expected)
			}
		})
	}
}

func TestECCCClient_FetchAlerts(t *testing.T) {
	fixture := `{
		"type": "FeatureCollection",
		"features": [
			{
				"properties": {
					"event": "SNOWFALL WARNING",
					"severity": "Moderate",
					"zone": "BC_14_09"
				}
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fixture))
	}))
	defer server.Close()

	client := &ECCCClient{
		client:  server.Client(),
		baseURL: server.URL,
	}

	alerts, err := client.FetchAlerts(context.Background(), []string{"BC_14_09"})
	if err != nil {
		t.Fatalf("FetchAlerts failed: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
}

func TestVancouverTuning_SnowPreAlert(t *testing.T) {
	alert := WeatherAlert{
		Type:  Snow,
		Event: "SNOWFALL WARNING",
	}

	params := VancouverTuning(alert)
	if !params.PreAlertEnabled {
		t.Errorf("expected PreAlertEnabled to be true for snow")
	}
}

func TestVancouverTuning_FogDurationWeight(t *testing.T) {
	alert := WeatherAlert{
		Type: Fog,
	}

	params := VancouverTuning(alert)
	if params.DurationWeight != 1.5 { // Assuming 1.5 as heavy weight
		t.Errorf("expected heavy duration weight for fog, got %f", params.DurationWeight)
	}
}
