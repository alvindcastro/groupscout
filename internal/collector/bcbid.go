package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// BCBidCollector processes award notices from BC Bid.
// Since the new BC Bid portal is JavaScript-heavy and lacks a public RSS feed,
// this collector is designed for a hybrid approach:
// 1. External trigger (e.g. n8n with Playwright) scrapes the award notice.
// 2. The raw HTML or JSON is passed to the /run endpoint in the RawData map.
// 3. This collector parses that raw data (potentially via Claude) into RawProjects.
type BCBidCollector struct {
	Verbose bool
}

// NewBCBidCollector returns a new BCBidCollector.
func NewBCBidCollector() *BCBidCollector {
	return &BCBidCollector{}
}

// Name satisfies the Collector interface.
func (b *BCBidCollector) Name() string { return "bcbid" }

// Collect satisfies the Collector interface.
// For BC Bid, we primarily expect data to be passed in via the context or a side-channel,
// as a direct scrape is restricted by the portal's architecture.
func (b *BCBidCollector) Collect(ctx context.Context) ([]RawProject, error) {
	// Look for raw data in the context if provided by the caller (e.g. from an HTTP request body).
	raw, ok := ctx.Value("bcbid_raw_input").(string)
	if !ok || raw == "" {
		if b.Verbose {
			log.Println("[bcbid] no raw input provided in context; skipping collection")
		}
		return nil, nil
	}

	// For now, we assume the raw input is a JSON array of award objects
	// or a single award object, likely pre-processed by a scraper or Claude.
	// If it's raw HTML, we would eventually pass it to Claude in the enrichment phase.

	// Implementation note: In a true 'AI-first' collector, we might store the
	// raw HTML in the RawProject and let the Enricher use Claude to extract fields.

	if strings.HasPrefix(strings.TrimSpace(raw), "<") {
		// It's HTML. Wrap it in a single RawProject for the Enricher to handle.
		return []RawProject{
			{
				Source:      "bcbid",
				ExternalID:  fmt.Sprintf("html-%d", time.Now().Unix()),
				Title:       "BC Bid Award Notice (HTML)",
				Description: "Raw HTML award notice from BC Bid portal.",
				IssuedAt:    time.Now(),
				RawData:     map[string]any{"raw_html": raw},
				Hash:        b.hashRaw(raw),
			},
		}, nil
	}

	// Attempt to parse as JSON
	var awards []map[string]any
	if err := json.Unmarshal([]byte(raw), &awards); err != nil {
		// Try as single object
		var single map[string]any
		if err2 := json.Unmarshal([]byte(raw), &single); err2 == nil {
			awards = []map[string]any{single}
		} else {
			return nil, fmt.Errorf("bcbid: failed to parse input as JSON or HTML: %w", err)
		}
	}

	var projects []RawProject
	for _, a := range awards {
		p := b.mapToRawProject(a)
		if p.ExternalID == "" {
			continue
		}
		p.Hash = b.hashRaw(fmt.Sprintf("%s|%s|%s", p.ExternalID, p.Title, p.SourceURL))
		projects = append(projects, p)
	}

	return projects, nil
}

func (b *BCBidCollector) mapToRawProject(m map[string]any) RawProject {
	getString := func(k string) string {
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}

	id := getString("opportunity_id")
	if id == "" {
		id = getString("id")
	}

	title := getString("title")
	if title == "" {
		title = getString("description")
	}

	return RawProject{
		Source:      "bcbid",
		ExternalID:  id,
		Title:       title,
		Location:    getString("location"),
		Value:       b.parseValue(m["value"]),
		Description: getString("description"),
		IssuedAt:    b.parseDate(getString("award_date")),
		SourceURL:   getString("url"),
		RawData:     m,
	}
}

func (b *BCBidCollector) parseValue(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case string:
		// Basic numeric cleanup
		s := strings.ReplaceAll(val, ",", "")
		s = strings.ReplaceAll(s, "$", "")
		if dot := strings.Index(s, "."); dot != -1 {
			s = s[:dot]
		}
		var n int64
		fmt.Sscanf(s, "%d", &n)
		return n
	}
	return 0
}

func (b *BCBidCollector) parseDate(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	// Try common formats
	formats := []string{"2006-01-02", "Jan 02, 2006", "01/02/2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func (b *BCBidCollector) hashRaw(s string) string {
	h := sha256.Sum256([]byte("bcbid|" + s))
	return fmt.Sprintf("%x", h)
}
