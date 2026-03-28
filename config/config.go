package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL             string
	ClaudeAPIKey            string
	SlackWebhookURL         string
	SendGridAPIKey          string
	RichmondPermitsURL      string
	BCBidRSSURL             string
	NewsAPIKey              string
	EnrichmentEnabled       bool
	PriorityAlertThreshold  int
	DigestDay               string
	DigestHour              int
}

// Load reads config from environment variables, falling back to sensible defaults.
func Load() (*Config, error) {
	return &Config{
		DatabaseURL:            getEnv("DATABASE_URL", "blockscout.db"),
		ClaudeAPIKey:           os.Getenv("CLAUDE_API_KEY"),
		SlackWebhookURL:        os.Getenv("SLACK_WEBHOOK_URL"),
		SendGridAPIKey:         os.Getenv("SENDGRID_API_KEY"),
		RichmondPermitsURL:     os.Getenv("RICHMOND_PERMITS_URL"),
		BCBidRSSURL:            os.Getenv("BCBID_RSS_URL"),
		NewsAPIKey:             os.Getenv("NEWS_API_KEY"),
		EnrichmentEnabled:      getEnv("ENRICHMENT_ENABLED", "true") == "true",
		PriorityAlertThreshold: getEnvInt("PRIORITY_ALERT_THRESHOLD", 9),
		DigestDay:              getEnv("DIGEST_DAY", "monday"),
		DigestHour:             getEnvInt("DIGEST_HOUR", 8),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
