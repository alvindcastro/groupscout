package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SlackAlerter struct {
	botToken string
	channel  string
	client   *http.Client
	baseURL  string
}

func NewSlackAlerter(botToken, channel string) *SlackAlerter {
	return &SlackAlerter{
		botToken: botToken,
		channel:  channel,
		client:   &http.Client{Timeout: 10 * time.Second},
		baseURL:  "https://slack.com/api",
	}
}

func (a *SlackAlerter) PostMessage(ctx context.Context, msg AlertMessage) (string, error) {
	blocks := buildBlocks(msg)
	payload := map[string]interface{}{
		"channel": a.channel,
		"blocks":  blocks,
		"text":    fmt.Sprintf("Airport Disruption Alert: %s", msg.AirportCode),
	}
	return a.send(ctx, "/chat.postMessage", payload)
}

func (a *SlackAlerter) UpdateMessage(ctx context.Context, ts string, msg AlertMessage) error {
	blocks := buildBlocks(msg)
	payload := map[string]interface{}{
		"channel": a.channel,
		"ts":      ts,
		"blocks":  blocks,
		"text":    fmt.Sprintf("Airport Disruption Update: %s", msg.AirportCode),
	}
	_, err := a.send(ctx, "/chat.update", payload)
	return err
}

func (a *SlackAlerter) SendResolve(ctx context.Context, ts string, summary ResolveSummary) error {
	text := fmt.Sprintf("✅ *Disruption Resolved: %s*\nTotal Duration: %d min\nFinal SPS Score: %.1f",
		summary.AirportCode, summary.TotalDuration, summary.FinalSPS)

	payload := map[string]interface{}{
		"channel":   a.channel,
		"thread_ts": ts,
		"text":      text,
	}
	_, err := a.send(ctx, "/chat.postMessage", payload)
	return err
}

func (a *SlackAlerter) send(ctx context.Context, endpoint string, payload interface{}) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := a.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+a.botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(respBody, &slackResp); err != nil {
		return "", err
	}

	if !slackResp.OK {
		return "", fmt.Errorf("slack error: %s", slackResp.Error)
	}

	return slackResp.TS, nil
}

func buildBlocks(msg AlertMessage) []interface{} {
	roomsText := fmt.Sprintf("%d", msg.RoomsAvail)
	if msg.RoomsAvail == 0 {
		roomsText = "room count not set — use /inventory N"
	}

	blocks := []interface{}{
		map[string]interface{}{
			"type": "header",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": fmt.Sprintf("🚨 Disruption Alert: %s", msg.AirportCode),
			},
		},
		map[string]interface{}{
			"type": "section",
			"fields": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Status:*\n%s", msg.State),
				},
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Cause:*\n%s", msg.Cause),
				},
			},
		},
		map[string]interface{}{
			"type": "section",
			"fields": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Impact:*\n%d cancellations (%d total flights)", msg.Cancelled, msg.TotalFlights),
				},
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Est. Stranded Pax:*\n%d", msg.EstStranded),
				},
			},
		},
		map[string]interface{}{
			"type": "section",
			"fields": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Earliest Recovery:*\n%s", msg.EarliestClear),
				},
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*Rooms Available:*\n%s", roomsText),
				},
			},
		},
		map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Suggested Actions:*\n• %s", strings.Join(msg.Actions, "\n• ")),
			},
		},
	}
	return blocks
}
