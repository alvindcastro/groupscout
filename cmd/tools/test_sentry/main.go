package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/getsentry/sentry-go"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.SentryDSN == "" {
		fmt.Println("SENTRY_DSN not set. Set it to test real delivery.")
	}

	logger.Init(cfg.JSONLog, cfg.SentryDSN)

	if cfg.SentryDSN != "" {
		defer sentry.Flush(2 * time.Second)
		fmt.Println("Sentry initialized. Sending test error...")
		sentry.CaptureMessage("Test Sentry Integration from GroupScout")
		sentry.CaptureException(fmt.Errorf("test error: Sentry is working"))
	} else {
		fmt.Println("Sentry NOT initialized (no DSN provided).")
	}

	fmt.Println("Test script finished.")
}
