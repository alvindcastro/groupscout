package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/alert"
	"github.com/alvindcastro/groupscout/internal/aviation"
	"github.com/alvindcastro/groupscout/internal/weather"
)

type hotelState struct {
	config  config.HotelConfig
	manager *alert.LifecycleManager
}

func main() {
	configPath := flag.String("config", "config/airports.yaml", "Path to airports config file")
	slackToken := flag.String("slack-token", os.Getenv("SLACK_BOT_TOKEN"), "Slack bot token")
	flag.Parse()

	if *slackToken == "" {
		log.Fatal("SLACK_BOT_TOKEN is required")
	}

	// Override from env if set
	if envPath := os.Getenv("ALERTD_CONFIG_PATH"); envPath != "" {
		*configPath = envPath
	}

	hotelConfigs, err := config.LoadAirportConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load hotel config: %v", err)
	}

	ecccClient := weather.NewECCCClient()
	yvrScraper := aviation.NewYVRScraper()
	navCanadaClient := aviation.NewNavCanadaClient()

	// Map to store managers per hotel/channel
	var hotels []hotelState
	for _, hc := range hotelConfigs {
		notifier := alert.NewSlackAlerter(*slackToken, hc.SlackChannel)
		hotels = append(hotels, hotelState{
			config:  hc,
			manager: alert.NewLifecycleManager(notifier),
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	errorCount := 0
	activeAlert := false

	log.Println("Starting alertd poll loop...")

	for {
		// Run poll
		active, err := runPoll(ctx, ecccClient, yvrScraper, navCanadaClient, hotels)
		if err != nil {
			errorCount++
			log.Printf("Poll error (count %d): %v", errorCount, err)
		} else {
			errorCount = 0
			activeAlert = active
		}

		interval := getPollInterval(activeAlert)
		if errorCount > 0 {
			interval = computeBackoff(errorCount, 10*time.Second, 5*time.Minute)
		}

		select {
		case <-ctx.Done():
			log.Println("Graceful shutdown complete.")
			return
		case <-time.After(interval):
			// continue loop
		}
	}
}

func runPoll(ctx context.Context, eccc *weather.ECCCClient, yvr *aviation.YVRScraper, nav *aviation.NavCanadaClient, hotels []hotelState) (bool, error) {
	// Fetch data
	// For now we assume YVR is the primary airport
	zones := []string{"BC_14_09", "BC_14_10", "BC_14_07"}
	weatherAlerts, err := eccc.FetchAlerts(ctx, zones)
	if err != nil {
		return false, fmt.Errorf("weather fetch failed: %w", err)
	}

	flightStatus, err := yvr.FetchStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("yvr status fetch failed: %w", err)
	}

	groundStop, err := nav.FetchGroundStop(ctx, "CYVR")
	if err != nil {
		// Log but don't fail the whole poll if NOTAMs are down?
		log.Printf("Warning: NavCanada fetch failed: %v", err)
	}

	var activeAny bool
	for _, h := range hotels {
		// Find relevant weather alert for YVR
		var relevantWeather *weather.WeatherAlert
		if len(weatherAlerts) > 0 {
			relevantWeather = &weatherAlerts[0] // Simplified
		}

		spsInput := aviation.SPSInput{
			CancelledCount:     flightStatus.CancelledCount,
			TotalFlights:       flightStatus.TotalDepartures,
			CancellationRate:   flightStatus.CancellationRate,
			HourOfDay:          time.Now().Hour(),
			WeatherAlert:       relevantWeather,
			SingleRunwayOps:    groundStop, // Or map more accurately
			AvgSeatsPerFlight:  160,
			ConnectingPaxRatio: 0.58,
		}

		sps := aviation.ComputeSPS(spsInput)
		err := h.manager.Process(ctx, "CYVR", sps)
		if err != nil {
			log.Printf("Error processing hotel %s: %v", h.config.Name, err)
		}

		// Check if this hotel has an active alert (Alert or Updating state)
		// We'd need to expose this from LifecycleManager or DisruptionEvent
		// For simplicity, we check sps state
		if sps.State == aviation.SoftAlert || sps.State == aviation.HardAlert {
			activeAny = true
		}
	}

	return activeAny, nil
}

func getPollInterval(active bool) time.Duration {
	if active {
		return 90 * time.Second
	}
	return 10 * time.Minute
}

func computeBackoff(errors int, base time.Duration, max time.Duration) time.Duration {
	if errors <= 0 {
		return base
	}
	backoff := time.Duration(math.Pow(2, float64(errors-1))) * base
	if backoff > max {
		return max
	}
	return backoff
}
