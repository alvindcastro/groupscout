package notify

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/alvindcastro/blockscout/internal/storage"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type EmailNotifier struct {
	APIKey string
}

func NewEmailNotifier(apiKey string) *EmailNotifier {
	return &EmailNotifier{APIKey: apiKey}
}

func (n *EmailNotifier) SendWeeklyDigest(ctx context.Context, toEmail string, leads []storage.Lead) error {
	if n.APIKey == "" {
		return fmt.Errorf("SENDGRID_API_KEY not set")
	}

	html, err := generateDigestHTML(leads)
	if err != nil {
		return fmt.Errorf("generate html: %w", err)
	}

	from := mail.NewEmail("GroupScout", "alerts@groupscout.ai") // Replace with verified sender
	subject := fmt.Sprintf("Weekly Lead Digest - %s", time.Now().Format("Jan 02, 2006"))
	to := mail.NewEmail("Sandman Sales Team", toEmail)
	message := mail.NewSingleEmail(from, subject, to, "Please view in an HTML-enabled client.", html)

	client := sendgrid.NewSendClient(n.APIKey)
	_, err = client.Send(message)
	return err
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
