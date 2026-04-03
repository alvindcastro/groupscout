package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL            string
	ClaudeAPIKey           string
	SlackWebhookURL        string
	SendGridAPIKey         string
	RichmondPermitsURL     string
	DeltaPermitsURL        string
	CreativeBCEnabled      bool
	CreativeBCURL          string
	BCBidRSSURL            string
	NewsAPIKey             string
	EnrichmentEnabled      bool
	EnrichmentThreshold    int
	PriorityAlertThreshold int
	MinPermitValueCAD      int64
	DigestDay              string
	DigestHour             int
}

// Load reads config from environment variables, falling back to sensible defaults.
// It also loads a .env file from the current directory if one exists — values in
// the .env are only applied when the variable is not already set in the environment,
// so real environment variables always take precedence.
func Load() (*Config, error) {
	loadDotEnv(".env")
	return &Config{
		DatabaseURL:            getEnv("DATABASE_URL", "blockscout.db"),
		ClaudeAPIKey:           os.Getenv("CLAUDE_API_KEY"),
		SlackWebhookURL:        os.Getenv("SLACK_WEBHOOK_URL"),
		SendGridAPIKey:         os.Getenv("SENDGRID_API_KEY"),
		RichmondPermitsURL:     os.Getenv("RICHMOND_PERMITS_URL"),
		DeltaPermitsURL:        os.Getenv("DELTA_PERMITS_URL"),
		CreativeBCEnabled:      os.Getenv("CREATIVEBC_ENABLED") == "true",
		CreativeBCURL:          os.Getenv("CREATIVEBC_URL"),
		BCBidRSSURL:            os.Getenv("BCBID_RSS_URL"),
		NewsAPIKey:             os.Getenv("NEWS_API_KEY"),
		EnrichmentEnabled:      getEnv("ENRICHMENT_ENABLED", "true") == "true",
		EnrichmentThreshold:    getEnvInt("ENRICHMENT_THRESHOLD", 1),
		PriorityAlertThreshold: getEnvInt("PRIORITY_ALERT_THRESHOLD", 9),
		MinPermitValueCAD:      int64(getEnvInt("MIN_PERMIT_VALUE_CAD", 500_000)),
		DigestDay:              getEnv("DIGEST_DAY", "monday"),
		DigestHour:             getEnvInt("DIGEST_HOUR", 8),
	}, nil
}

// loadDotEnv reads key=value pairs from a file and sets them as environment
// variables. Lines starting with # and blank lines are ignored.
// Existing environment variables are never overwritten.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional — silence the error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
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
