package config

import (
	"os"
	"testing"
)

func TestLoadAirportConfig(t *testing.T) {
	content := `
- id: sandman-richmond
  name: Sandman Richmond
  slack_channel: "#alerts-yvr"
  airports:
    - code: CYVR
      distance_min: 8
  alert_threshold_sps: 60
  distressed_rate: 119
  rack_rate: 220
  airline_contacts:
    - carrier: Air Canada
      ops_phone: "604-276-7477"
`
	tmpfile, err := os.CreateTemp("", "airports*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	configs, err := LoadAirportConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadAirportConfig failed: %v", err)
	}

	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	c := configs[0]
	if c.Name != "Sandman Richmond" {
		t.Errorf("expected name Sandman Richmond, got %s", c.Name)
	}
	if len(c.Airports) == 0 || c.Airports[0].Code != "CYVR" {
		t.Errorf("expected airport CYVR, got %v", c.Airports)
	}
	if len(c.Airports) == 0 || c.Airports[0].DistanceMin != 8 {
		t.Errorf("expected distance 8, got %v", c.Airports)
	}
}

func TestLoadAirportConfig_AirlineContacts(t *testing.T) {
	content := `
- id: test
  airline_contacts:
    - carrier: Air Canada
      ops_phone: "604-276-7477"
`
	tmpfile, err := os.CreateTemp("", "airports*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	configs, err := LoadAirportConfig(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(configs[0].AirlineContacts) == 0 {
		t.Fatal("expected airline contacts")
	}
	if configs[0].AirlineContacts[0].OpsPhone != "604-276-7477" {
		t.Errorf("expected phone 604-276-7477, got %s", configs[0].AirlineContacts[0].OpsPhone)
	}
}

func TestLoadAirportConfig_MissingFile(t *testing.T) {
	_, err := LoadAirportConfig("non-existent-file.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
