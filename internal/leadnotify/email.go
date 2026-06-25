package leadnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

const resendAPIURL = "https://api.resend.com/emails"

// defaultEmailFrom is used when no From address is configured. The domain must be
// verified in the Resend account; otherwise Resend rejects sends with HTTP 403.
const defaultEmailFrom = "GroupScout <alerts@groupscout.ai>"

type EmailNotifier struct {
	APIKey string
	From   string
}

// NewEmailNotifier returns a notifier. An empty from falls back to defaultEmailFrom.
func NewEmailNotifier(apiKey, from string) *EmailNotifier {
	if from == "" {
		from = defaultEmailFrom
	}
	return &EmailNotifier{APIKey: apiKey, From: from}
}

func (n *EmailNotifier) SendWeeklyDigest(ctx context.Context, toEmail string, leads []storage.Lead) error {
	if n.APIKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	html, err := generateDigestHTML(leads)
	if err != nil {
		return fmt.Errorf("generate html: %w", err)
	}

	return n.post(ctx, map[string]any{
		"from":    n.From,
		"to":      []string{toEmail},
		"subject": fmt.Sprintf("Weekly Lead Digest - %s", time.Now().Format("Jan 02, 2006")),
		"html":    html,
	})
}

// SendLeads emails the given leads to every recipient in a single message.
// It mirrors the leads delivered to Slack so operators get an email copy.
// Returns nil immediately when there are no recipients or no leads.
func (n *EmailNotifier) SendLeads(ctx context.Context, recipients []string, leads []storage.Lead) error {
	if len(recipients) == 0 || len(leads) == 0 {
		return nil
	}
	if n.APIKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	html, err := generateDigestHTML(leads)
	if err != nil {
		return fmt.Errorf("generate html: %w", err)
	}

	subject := fmt.Sprintf("New GroupScout Lead: %s", leads[0].Title)
	if len(leads) > 1 {
		subject = fmt.Sprintf("GroupScout: %d new leads - %s", len(leads), time.Now().Format("Jan 02, 2006"))
	}

	return n.post(ctx, map[string]any{
		"from":    n.From,
		"to":      recipients,
		"subject": subject,
		"html":    html,
	})
}

// SendNotice emails a short status message to every recipient. It mirrors
// non-lead cadence outcomes (e.g. "no new leads today") that are also posted to
// Slack, so email recipients stay in sync with Slack. No-ops when there are no
// recipients.
func (n *EmailNotifier) SendNotice(ctx context.Context, recipients []string, subject, message string) error {
	if len(recipients) == 0 || message == "" {
		return nil
	}
	if n.APIKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	html := fmt.Sprintf("<p style=\"font-family: sans-serif; line-height: 1.6; color: #333;\">%s</p>",
		template.HTMLEscapeString(message))

	return n.post(ctx, map[string]any{
		"from":    n.From,
		"to":      recipients,
		"subject": subject,
		"text":    message,
		"html":    html,
	})
}

// post marshals the payload and sends it to the Resend API, returning an error
// for any non-2xx response.
func (n *EmailNotifier) post(ctx context.Context, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func generateDigestHTML(leads []storage.Lead) (string, error) {
	const tpl = `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: sans-serif; line-height: 1.6; color: #333; }
        .lead-card { border: 1px solid #ddd; padding: 15px; margin-bottom: 20px; border-radius: 5px; }
        .high-priority { border-left: 5px solid #d9534f; }
        .medium-priority { border-left: 5px solid #f0ad4e; }
        .score { font-weight: bold; font-size: 1.2em; color: #d9534f; }
        .meta { font-size: 0.9em; color: #666; }
        h2 { margin-top: 0; }
        .btn { display: inline-block; padding: 10px 15px; background: #0275d8; color: #fff; text-decoration: none; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>Weekly High-Priority Leads</h1>
    <p>Here are the top construction and event leads for the past week.</p>

    {{range .}}
    <div class="lead-card {{if ge .PriorityScore 8}}high-priority{{else}}medium-priority{{end}}">
        <h2>{{.Title}}</h2>
        <div class="score">Priority Score: {{.PriorityScore}}/10</div>
        <p><strong>Location:</strong> {{.Location}}</p>
        <p><strong>Project Type:</strong> {{.ProjectType}}</p>
        <p><strong>Reason:</strong> {{.PriorityReason}}</p>
        <p><strong>Notes:</strong> {{.Notes}}</p>
        <div class="meta">
            Source: {{.Source}} | Value: ${{.ProjectValue}} | GC: {{.GeneralContractor}}
        </div>
        <br>
        <a href="{{.SourceURL}}" class="btn">View Source</a>
    </div>
    {{end}}
</body>
</html>`

	t, err := template.New("digest").Parse(tpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, displayLeads(leads)); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func displayLeads(leads []storage.Lead) []storage.Lead {
	out := make([]storage.Lead, len(leads))
	copy(out, leads)
	for i := range out {
		out[i].PriorityScore = displayPriorityScore(out[i].PriorityScore)
	}
	return out
}

func displayPriorityScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score > 10:
		return 10
	default:
		return score
	}
}
