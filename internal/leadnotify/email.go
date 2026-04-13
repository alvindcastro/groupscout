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

type EmailNotifier struct {
	APIKey string
}

func NewEmailNotifier(apiKey string) *EmailNotifier {
	return &EmailNotifier{APIKey: apiKey}
}

func (n *EmailNotifier) SendWeeklyDigest(ctx context.Context, toEmail string, leads []storage.Lead) error {
	if n.APIKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	html, err := generateDigestHTML(leads)
	if err != nil {
		return fmt.Errorf("generate html: %w", err)
	}

	payload := map[string]any{
		"from":    "GroupScout <alerts@groupscout.ai>",
		"to":      []string{toEmail},
		"subject": fmt.Sprintf("Weekly Lead Digest - %s", time.Now().Format("Jan 02, 2006")),
		"html":    html,
	}

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
	if err := t.Execute(&buf, leads); err != nil {
		return "", err
	}

	return buf.String(), nil
}
