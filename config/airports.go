package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type HotelConfig struct {
	ID                string           `yaml:"id"`
	Name              string           `yaml:"name"`
	SlackChannel      string           `yaml:"slack_channel"`
	Airports          []AirportRef     `yaml:"airports"`
	AlertThresholdSPS float64          `yaml:"alert_threshold_sps"`
	DistressedRate    int              `yaml:"distressed_rate"`
	RackRate          int              `yaml:"rack_rate"`
	AirlineContacts   []AirlineContact `yaml:"airline_contacts"`
}

type AirportRef struct {
	Code        string `yaml:"code"` // CYVR, CYXX
	DistanceMin int    `yaml:"distance_min"`
}

type AirlineContact struct {
	Carrier  string `yaml:"carrier"`
	OpsPhone string `yaml:"ops_phone"`
}

func LoadAirportConfig(path string) ([]HotelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var configs []HotelConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}
