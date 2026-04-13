package leadnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

// Notifier is the interface every notification channel must implement.
// The pipeline calls Send once per run with all new leads.
type Notifier interface {
	Send(ctx context.Context, leads []storage.Lead) error
}

// SlackNotifier posts a digest of leads to a Slack incoming webhook.
type SlackNotifier struct {
	WebhookURL string
	BaseURL    string
	client     *http.Client
}

// NewSlackNotifier returns a SlackNotifier with a 10-second HTTP timeout.
func NewSlackNotifier(webhookURL, baseURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
		BaseURL:    baseURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts all leads to the Slack webhook as a single digest message.
// Returns nil immediately if leads is empty.
func (s *SlackNotifier) Send(ctx context.Context, leads []storage.Lead) error {
	if len(leads) == 0 {
		return nil
	}

	payload := s.buildMessage(leads)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack: webhook returned HTTP %d: %s", resp.StatusCode, raw)
	}

	return nil
}

// buildMessage formats leads into a Slack Block Kit payload.
// Header block shows the run date and lead count.
// Each lead gets its own section block with key fields and a priority score emoji.
func (s *SlackNotifier) buildMessage(leads []storage.Lead) map[string]any {
	blocks := []map[string]any{
		headerBlock(len(leads)),
		dividerBlock(),
	}

	for _, l := range leads {
		blocks = append(blocks, s.leadBlock(l), dividerBlock())
	}

	return map[string]any{"blocks": blocks}
}

func headerBlock(n int) map[string]any {
	label := "lead"
	if n != 1 {
		label = "leads"
	}
	return map[string]any{
		"type": "header",
		"text": map[string]any{
			"type": "plain_text",
			"text": fmt.Sprintf("🏗️  groupscout — %d new %s  (%s)", n, label, time.Now().Format("Jan 2, 2006")),
		},
	}
}

func dividerBlock() map[string]any {
	return map[string]any{"type": "divider"}
}

// leadBlock renders one Lead as a Slack section block.
func (s *SlackNotifier) leadBlock(l storage.Lead) map[string]any {
	contactLine := ""
	if l.Contractor != "" || l.Applicant != "" {
		contactLine = "\n📞"
		if l.Contractor != "" {
			contactLine += fmt.Sprintf(" *Contractor:* %s", l.Contractor)
		}
		if l.Applicant != "" {
			if l.Contractor != "" {
				contactLine += "  |"
			}
			contactLine += fmt.Sprintf(" *Applicant:* %s", l.Applicant)
		}
	}

	sourceLine := ""
	if l.SourceURL != "" {
		sourceLine = fmt.Sprintf("\n📄 <%s|View source document>", l.SourceURL)
	}

	auditLine := ""
	if s.BaseURL != "" && l.RawInputID != "" {
		auditURL := fmt.Sprintf("%s/leads/%s/raw", s.BaseURL, l.ID)
		auditLine = fmt.Sprintf("\n🔍 <%s|View raw audit data>", auditURL)
	}

	text := fmt.Sprintf("*%s*\t\t\t%s *Score: %d/10*\n"+
		"📍 %s  |  💰 $%s CAD  |  🏢 GC: %s%s\n"+
		"🔌 *Source:* %s\n"+
		"🕐 *Outreach:* %s\n"+
		"📝 %s%s%s",
		l.Title,
		scoreEmoji(l.PriorityScore), l.PriorityScore,
		l.Location,
		formatCAD(l.ProjectValue),
		l.GeneralContractor,
		contactLine,
		l.Source,
		l.SuggestedOutreachTiming,
		l.Notes,
		sourceLine,
		auditLine,
	)

	fields := []map[string]any{
		markdownField(fmt.Sprintf("*Type:* %s", l.ProjectType)),
		markdownField(fmt.Sprintf("*Crew:* ~%d  |  ~%d months", l.EstimatedCrewSize, l.EstimatedDurationMonths)),
		markdownField(fmt.Sprintf("*Out-of-town:* %s", boolLabel(l.OutOfTownCrewLikely))),
		markdownField(fmt.Sprintf("*Reason:* %s", l.PriorityReason)),
	}

	if l.Rationale != "" {
		fields = append(fields, markdownField(fmt.Sprintf("*Rationale:* %s", l.Rationale)))
	}

	return map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": text,
		},
		"fields": fields,
	}
}

func markdownField(s string) map[string]any {
	return map[string]any{"type": "mrkdwn", "text": s}
}

func scoreEmoji(score int) string {
	switch {
	case score >= 9:
		return "🔥"
	case score >= 7:
		return "⚡"
	case score >= 5:
		return "👀"
	default:
		return "📌"
	}
}

func boolLabel(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func formatCAD(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
