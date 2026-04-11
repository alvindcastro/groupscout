package ollama

import (
	"context"
	"fmt"
	"math"
	"time"
)

// DisruptionEvent matches the required fields for alert generation.
type DisruptionEvent struct {
	Cause            string    `json:"cause"`
	SPS              float64   `json:"sps"`
	AlertState       string    `json:"alert_state"` // "watch" | "soft_alert" | "hard_alert" | "resolve"
	FlightsCancelled int       `json:"flights_cancelled"`
	FlightsScheduled int       `json:"flights_scheduled"`
	EstStranded      int       `json:"est_stranded"`
	StartedAt        time.Time `json:"started_at"`
	RecoveryETA      time.Time `json:"recovery_eta"` // zero if unknown
}

// AlertCopyGenerator uses an LLM to generate actionable disruption alert copy.
type AlertCopyGenerator struct {
	client LLMClient
}

// NewAlertCopyGenerator returns a new AlertCopyGenerator.
func NewAlertCopyGenerator(client LLMClient) *AlertCopyGenerator {
	return &AlertCopyGenerator{client: client}
}

// Generate builds a prompt and calls the LLM to create alert copy.
func (g *AlertCopyGenerator) Generate(ctx context.Context, event DisruptionEvent) (string, error) {
	ctx = WithUseCase(ctx, "alert_copy")
	systemPrompt := `
You are a hotel operations assistant at a Vancouver-area airport hotel (near YVR).
You receive real-time flight disruption data and write 2–3 actionable bullet points
for the front desk manager. 

Rules:
- Be specific: name the cause (e.g., atmospheric river, fog), estimated passenger count,
  and earliest recovery time.
- Include a dollar or revenue angle: e.g., "pull back OTA inventory", "activate distressed rate".
- Adjust tone by time of day: assertive urgency after 9pm, calm preparation before 6pm.
- Never use generic filler phrases like "stay informed" or "monitor the situation".
- Output plain text only. Three bullet points maximum. No markdown headers.
`

	userPrompt := g.buildUserPrompt(event)

	resp, err := g.client.ChatComplete(ctx, systemPrompt, userPrompt)
	if err != nil {
		// Fallback: empty copy, let the caller use hardcoded template
		return "", nil
	}

	return resp, nil
}

func (g *AlertCopyGenerator) buildUserPrompt(event DisruptionEvent) string {
	duration := time.Since(event.StartedAt).Round(15 * time.Minute)
	if event.StartedAt.IsZero() {
		duration = 0
	}

	cancelPct := 0.0
	if event.FlightsScheduled > 0 {
		cancelPct = (float64(event.FlightsCancelled) / float64(event.FlightsScheduled)) * 100
	}

	tod := "morning"
	hour := time.Now().Hour()
	switch {
	case hour >= 21 || hour < 5:
		tod = "overnight"
	case hour >= 17:
		tod = "evening"
	case hour >= 12:
		tod = "afternoon"
	}

	recovery := "unknown"
	if !event.RecoveryETA.IsZero() {
		hours := math.Round(time.Until(event.RecoveryETA).Hours())
		recovery = fmt.Sprintf("%.0f hours from now", hours)
	}

	return fmt.Sprintf(`
Event Details:
- Cause: %s
- Alert State: %s
- SPS Score: %.1f
- Duration so far: %v
- Cancellation Rate: %.1f%% (%d/%d flights)
- Estimated Stranded Pax: %d
- Current Time of Day: %s
- Recovery ETA: %s
`, event.Cause, event.AlertState, event.SPS, duration, cancelPct, event.FlightsCancelled, event.FlightsScheduled, event.EstStranded, tod, recovery)
}
