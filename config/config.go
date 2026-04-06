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
	GeminiAPIKey           string
	AIProvider             string
	SlackWebhookURL        string
	ResendAPIKey           string
	RichmondPermitsURL     string
	DeltaPermitsURL        string
	CreativeBCEnabled      bool
	CreativeBCURL          string
	VCCEnabled             bool
	VCCURL                 string
	BCBidEnabled           bool
	BCBidRSSURL            string
	NewsEnabled            bool
	NewsRSSURL             string
	AnnouncementsEnabled   bool
	EventbriteEnabled      bool
	EventbriteURL          string
	EnrichmentEnabled      bool
	EnrichmentThreshold    int
	PriorityAlertThreshold int
	MinPermitValueCAD      int64
	Port                   int
	APIToken               string
	DigestDay              string
	DigestHour             int
	JSONLog                bool
	SentryDSN              string
}

// Load reads config from environment variables, falling back to sensible defaults.
// It also loads a .env file from the current directory if one exists — values in
// the .env are only applied when the variable is not already set in the environment,
// so real environment variables always take precedence.
func Load() (*Config, error) {
	loadDotEnv(".env")
	return &Config{
		DatabaseURL:            getEnv("DATABASE_URL", "groupscout.db"),
		ClaudeAPIKey:           os.Getenv("CLAUDE_API_KEY"),
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		AIProvider:             getEnv("AI_PROVIDER", "claude"),
		SlackWebhookURL:        os.Getenv("SLACK_WEBHOOK_URL"),
		ResendAPIKey:           os.Getenv("RESEND_API_KEY"),
		RichmondPermitsURL:     os.Getenv("RICHMOND_PERMITS_URL"),
		DeltaPermitsURL:        os.Getenv("DELTA_PERMITS_URL"),
		CreativeBCEnabled:      os.Getenv("CREATIVEBC_ENABLED") == "true",
		CreativeBCURL:          os.Getenv("CREATIVEBC_URL"),
		VCCEnabled:             getEnv("VCC_ENABLED", "false") == "true",
		VCCURL:                 os.Getenv("VCC_URL"),
		BCBidEnabled:           getEnv("BCBID_ENABLED", "true") == "true",
		BCBidRSSURL:            getEnv("BCBID_RSS_URL", "https://www.civicinfo.bc.ca/rss/bids-bt.php?id=14,https://www.civicinfo.bc.ca/rss/bids-bt.php?id=53"),
		NewsEnabled:            os.Getenv("NEWS_ENABLED") == "true",
		NewsRSSURL:             getEnv("NEWS_RSS_URL", "https://news.google.com/rss/search?q=%22Richmond+BC%22+construction+OR+%22Metro+Vancouver%22+infrastructure+contract+awarded+OR+%22YVR%22+expansion&hl=en-CA&gl=CA&ceid=CA:en"),
		AnnouncementsEnabled:   getEnv("ANNOUNCEMENTS_ENABLED", "true") == "true",
		EventbriteEnabled:      getEnv("EVENTBRITE_ENABLED", "true") == "true",
		EventbriteURL:          getEnv("EVENTBRITE_URL", "https://www.eventbrite.ca/d/canada--vancouver/professional-services--events/"),
		EnrichmentEnabled:      getEnv("ENRICHMENT_ENABLED", "true") == "true",
		EnrichmentThreshold:    getEnvInt("ENRICHMENT_THRESHOLD", 1),
		PriorityAlertThreshold: getEnvInt("PRIORITY_ALERT_THRESHOLD", 9),
		MinPermitValueCAD:      int64(getEnvInt("MIN_PERMIT_VALUE_CAD", 500_000)),
		Port:                   getEnvInt("PORT", 8080),
		APIToken:               os.Getenv("API_TOKEN"),
		DigestDay:              getEnv("DIGEST_DAY", "monday"),
		DigestHour:             getEnvInt("DIGEST_HOUR", 9),
		JSONLog:                getEnv("JSON_LOG", "false") == "true",
		SentryDSN:              os.Getenv("SENTRY_DSN"),
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
