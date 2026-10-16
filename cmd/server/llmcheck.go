package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/enrichment"
	"github.com/alvindcastro/groupscout/internal/llm"
)

// runLLMCheck sends sample projects to each AI provider on its own, with no
// failover, and prints whether each reply is a usable lead. Run it after
// adding a key or changing a model:
//
//	groupscout llm-check            # every provider in LLM_PROVIDERS
//	groupscout llm-check openai     # only the providers named
//
// Each sample is one real API call. It returns the exit code: 0 when every
// provider checked passed every sample.
func runLLMCheck(cfg *config.Config, only []string, out io.Writer) int {
	settings := llmSettings(cfg)
	if len(only) > 0 {
		settings.Order = only
	}
	clients, skipped := llm.BuildClients(settings)
	for _, reason := range skipped {
		fmt.Fprintf(out, "SKIP  %s\n", reason)
	}
	if len(clients) == 0 {
		fmt.Fprintln(out, "no AI provider to check")
		return 1
	}

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	code := 0
	for _, c := range clients {
		chain := llm.NewChain(c)
		chain.Retries = 1
		chain.Logger = quiet
		enricher := enrichment.NewLLMEnricher(chain)

		for _, sample := range llmCheckSamples() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			start := time.Now()
			lead, err := enricher.Enrich(ctx, sample)
			cancel()
			took := time.Since(start).Seconds()
			if err != nil {
				code = 1
				fmt.Fprintf(out, "FAIL  %-7s %-26s %-17s %5.1fs  %v\n", c.Provider(), c.Model(), sample.Source, took, err)
				if llm.KindOf(err) == llm.KindAccount || !chain.Status()[0].Available {
					fmt.Fprintf(out, "      %s: skipping its remaining samples\n", c.Provider())
					break
				}
				continue
			}
			fmt.Fprintf(out, "ok    %-7s %-26s %-17s %5.1fs  score %d, %s, crew %d, out-of-town %t\n",
				c.Provider(), c.Model(), sample.Source, took,
				lead.PriorityScore, lead.ProjectType, lead.EstimatedCrewSize, lead.OutOfTownCrewLikely)
		}
	}
	return code
}

// llmCheckSamples are fictional projects in the shape of three real sources:
// a permit, a film production and a convention.
func llmCheckSamples() []collector.RawProject {
	return []collector.RawProject{
		{
			Source:      "richmond_permits",
			Title:       "New industrial warehouse with office mezzanine",
			Location:    "12000 Horseshoe Way, Richmond BC",
			Value:       12_500_000,
			Description: "Construction of a 140,000 sq ft tilt-up warehouse. Contractor: Northbridge Industrial Builders Ltd (Calgary, AB).",
			IssuedAt:    time.Now().AddDate(0, 0, -5),
		},
		{
			Source:      "creativebc",
			Title:       "Harbour Lights (Season 2)",
			Description: "TV Series, 10 episodes. Studio: a Los Angeles streaming network.",
			Metadata: map[string]any{
				"schedule": time.Now().AddDate(0, 0, 21).Format("January 2, 2006") + " - " + time.Now().AddDate(0, 6, 0).Format("January 2, 2006"),
				"address":  "Richmond, BC",
				"manager":  "Production Manager (example)",
			},
		},
		{
			Source:      "vcc_events",
			Title:       "Pacific Northwest Mining and Exploration Congress",
			Description: "Four-day international congress, about 2,500 delegates. " + time.Now().AddDate(0, 1, 0).Format("January 2, 2006"),
			SourceURL:   "https://www.vancouverconventioncentre.com/events",
		},
	}
}
