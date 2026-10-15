// Package heartbeat pings an external dead man's switch after each lead
// cadence run (IaC Phase 9, groupscout-site-6ah.10).
//
// The check lives outside the stack (Healthchecks.io), so a cadence that never
// runs (n8n, the API, Podman or the whole PC down) still raises an alert: the
// check expects a ping on the cadence schedule and alerts when none arrives.
// Success is a request to the check URL itself and failure to URL + "/fail",
// which is the Healthchecks.io ping API. The URL embeds the ping key, so it is
// treated as a secret and never logged.
package heartbeat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxBody is the request body Healthchecks.io keeps per ping.
const maxBody = 10_000

// Pinger sends success and failure pings. The zero value and a Pinger built
// from an empty URL are disabled and do nothing.
type Pinger struct {
	url      string
	client   *http.Client
	attempts int
	backoff  time.Duration
}

// New returns a Pinger for checkURL; an empty checkURL disables pinging.
func New(checkURL string) (*Pinger, error) {
	checkURL = strings.TrimRight(strings.TrimSpace(checkURL), "/")
	if checkURL == "" {
		return &Pinger{}, nil
	}
	u, err := url.Parse(checkURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		// The URL holds the ping key: don't echo it back in the error.
		return nil, fmt.Errorf("heartbeat: check URL must be an absolute http(s) URL")
	}
	return &Pinger{
		url:      checkURL,
		client:   &http.Client{Timeout: 10 * time.Second},
		attempts: 3,
		backoff:  2 * time.Second,
	}, nil
}

// Enabled reports whether a check URL is configured.
func (p *Pinger) Enabled() bool { return p != nil && p.url != "" }

// Host returns the check URL's host for logging, without the ping key.
func (p *Pinger) Host() string {
	if !p.Enabled() {
		return ""
	}
	u, _ := url.Parse(p.url)
	return u.Host
}

// Success reports a completed cadence run. body is shown in the check's log.
func (p *Pinger) Success(ctx context.Context, body string) error {
	return p.send(ctx, "", body)
}

// Fail reports a failed cadence run so the check alerts right away instead
// of waiting for the grace period to run out.
func (p *Pinger) Fail(ctx context.Context, body string) error {
	return p.send(ctx, "/fail", body)
}

func (p *Pinger) send(ctx context.Context, suffix, body string) error {
	if !p.Enabled() {
		return nil
	}
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	var lastErr error
	for attempt := 1; attempt <= p.attempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("heartbeat: %w (last error: %v)", ctx.Err(), lastErr)
			case <-time.After(p.backoff * time.Duration(attempt-1)):
			}
		}
		lastErr = p.post(ctx, p.url+suffix, body)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (p *Pinger) post(ctx context.Context, target, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("heartbeat: build request")
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := p.client.Do(req)
	if err != nil {
		// *url.Error includes the URL (and so the ping key); keep only the cause.
		var cause error = err
		if ue, ok := err.(*url.Error); ok {
			cause = ue.Err
		}
		return fmt.Errorf("heartbeat: ping %s: %v", p.Host(), cause)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("heartbeat: ping %s: HTTP %d", p.Host(), resp.StatusCode)
	}
	return nil
}
