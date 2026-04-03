package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func init() {
	// Default logger - plain text for dev
	Log = slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func Init(json bool) {
	if json {
		Log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		Log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
}
