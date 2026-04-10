package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AlertType string

const (
	Snow             AlertType = "Snow"
	Fog              AlertType = "Fog"
	AtmosphericRiver AlertType = "AtmosphericRiver"
	Wind             AlertType = "Wind"
	Unknown          AlertType = "Unknown"
)

type WeatherAlert struct {
	Zone      string
	Event     string
	Severity  string // Minor | Moderate | Extreme
	Type      AlertType
	StartTime time.Time
}

type TuningParams struct {
	DurationWeight    float64
	PreAlertEnabled   bool
	SPSWatchThreshold float64
	LookbackHours     int
}

func VancouverTuning(alert WeatherAlert) TuningParams {
	params := TuningParams{
		DurationWeight:    1.0,
		PreAlertEnabled:   false,
		SPSWatchThreshold: 60.0,
		LookbackHours:     24,
	}

	switch alert.Type {
	case AtmosphericRiver:
		params.SPSWatchThreshold = 40.0
		params.LookbackHours = 48
	case Snow:
		params.PreAlertEnabled = true
	case Fog:
		params.DurationWeight = 1.5
	}

	return params
}

type ECCCClient struct {
	client  *http.Client
	baseURL string
}

func NewECCCClient() *ECCCClient {
	return &ECCCClient{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.weather.gc.ca/collections/alerts/items",
	}
}

func (c *ECCCClient) FetchAlerts(ctx context.Context, zones []string) ([]WeatherAlert, error) {
	var allAlerts []WeatherAlert

	for _, zone := range zones {
		url := fmt.Sprintf("%s?zone=%s", c.baseURL, zone)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue // Or return error? For now follow simple flow
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		alerts, err := parseWeatherAlerts(body)
		if err != nil {
			return nil, err
		}
		allAlerts = append(allAlerts, alerts...)
	}

	return allAlerts, nil
}

type geoJSONResponse struct {
	Features []struct {
		Properties struct {
			Event    string `json:"event"`
			Headline string `json:"headline"`
			Severity string `json:"severity"`
			Zone     string `json:"zone"`
		} `json:"properties"`
	} `json:"features"`
}

func parseWeatherAlerts(body []byte) ([]WeatherAlert, error) {
	var res geoJSONResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var alerts []WeatherAlert
	for _, f := range res.Features {
		p := f.Properties
		alerts = append(alerts, WeatherAlert{
			Zone:     p.Zone,
			Event:    p.Event,
			Severity: p.Severity,
			Type:     ClassifyAlertType(p.Event, p.Headline),
		})
	}
	return alerts, nil
}

func ClassifyAlertType(event, headline string) AlertType {
	event = strings.ToUpper(event)
	headline = strings.ToLower(headline)

	if strings.Contains(event, "SNOWFALL") || strings.Contains(event, "SNOW") {
		return Snow
	}
	if strings.Contains(event, "FOG") {
		return Fog
	}
	if strings.Contains(headline, "atmospheric") {
		return AtmosphericRiver
	}
	if strings.Contains(event, "WIND") {
		return Wind
	}

	return Unknown
}
