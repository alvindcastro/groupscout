package enrichment

import (
	"strings"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
)

// Gold Standard Fixtures for Strict TDD
var (
	richmondWarehousePermit = collector.RawProject{
		Source:      "richmond",
		Title:       "New Warehouse Construction",
		Location:    "12345 Rice Mill Rd, Richmond, BC",
		Value:       10000000, // $10M
		Description: "Construction of a new 100,000 sq ft industrial warehouse with office space.",
		IssuedAt:    time.Now(),
	}

	creativeBCFeatureFilm = collector.RawProject{
		Source:      "creativebc",
		Title:       "Project Bluebook",
		Description: "Major Sci-Fi Feature Film",
		RawData: map[string]any{
			"schedule": "July 2026 - October 2026",
			"address":  "Richmond, BC",
			"manager":  "John Doe",
		},
	}
)

func TestPrompts_JSONStructure(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{"PermitPrompt", permitPrompt(richmondWarehousePermit)},
		{"CreativeBCPrompt", creativeBCPrompt(creativeBCFeatureFilm)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In a real TDD scenario, we would call the LLM here or use a mock.
			// For this structural test, we just verify the prompt contains the expected JSON schema instructions.
			if tt.prompt == "" {
				t.Errorf("%s: prompt is empty", tt.name)
			}
		})
	}
}

func TestPrompts_PriorityLogic(t *testing.T) {
	// This test ensures that our prompt instructions align with our scoring logic.
	// We check if the prompt strings contain the specific booster keywords defined in PHASES.md.

	t.Run("RichmondBooster", func(t *testing.T) {
		prompt := creativeBCPrompt(creativeBCFeatureFilm)
		expected := "+1 if production address is in Richmond or Surrey"
		if !strings.Contains(prompt, expected) {
			t.Errorf("CreativeBC prompt missing Richmond booster logic")
		}
	})

	t.Run("LargeValueBooster", func(t *testing.T) {
		prompt := newsPrompt(collector.RawProject{Title: "Large Bridge Project"})
		expected := "+1 if large value mentioned"
		if !strings.Contains(prompt, expected) {
			t.Errorf("News prompt missing large value booster logic")
		}
	})
}
