package logger

import (
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
)

var Log *slog.Logger

func init() {
	// Default logger - plain text for dev
	Log = slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func Init(json bool, sentryDSN string) {
	if json {
		Log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		Log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	if sentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDSN,
			TracesSampleRate: 1.0,
		})
		if err != nil {
			Log.Error("sentry.Init failed", "error", err)
		} else {
			Log.Info("Sentry initialized")
		}
	}
}
