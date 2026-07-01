package events

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/logger"
)

// VCCCollector scrapes the Vancouver Convention Centre events page.
type VCCCollector struct {
	client  *http.Client
	url     string
	Verbose bool
}

// NewVCCCollector creates a new VCCCollector.
func NewVCCCollector(url string) *VCCCollector {
	if url == "" {
		url = "https://www.vancouverconventioncentre.com/events"
	}
	return &VCCCollector{
		client: &http.Client{Timeout: 30 * time.Second},
		url:    url,
	}
}

func (c *VCCCollector) Name() string {
	return "vcc_events"
}

func (c *VCCCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	if c.Verbose {
		logger.Log.Info("fetching vcc events", "url", c.url)
	}
	// Use a more standard User-Agent to avoid being blocked
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VCC events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, &collector.SourceDriftError{URL: c.url, StatusCode: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d for %s", resp.StatusCode, c.url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if c.Verbose {
		logger.Log.Debug("VCC response", "status", resp.StatusCode, "content_type", resp.Header.Get("Content-Type"))
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var projects []collector.RawProject
	// The VCC events page typically has event cards.
	// We'll need to inspect the actual markup to be sure, but let's start with a likely structure.
	// Based on common patterns and PHASES.md, we want to extract:
	// name, start date, end date, category tag.

	// The live VCC events page renders each event as an <a class="event-item">
	// anchor wrapping .event-details (title h2), .event-date, and .event-location.
	// The older selectors (.event-card/.views-row/etc.) are kept as fallbacks so
	// the collector survives another markup revision.
	doc.Find(".event-item, article, .event-card, .event, .listing-item, .views-row").Each(func(i int, sel *goquery.Selection) {
		s := sel.Find(".event-details h2, h2, h3, h4, .event-title, .title").First()
		title := strings.TrimSpace(s.Text())
		if title == "" || len(title) > 150 || len(title) < 5 {
			return
		}
		titleLower := strings.ToLower(title)
		if titleLower == "events" || strings.Contains(titleLower, "organize") || strings.Contains(titleLower, "attend") ||
			strings.Contains(titleLower, "news") || strings.Contains(titleLower, "about") || strings.Contains(titleLower, "contact") ||
			strings.Contains(titleLower, "career") || strings.Contains(titleLower, "privacy") || strings.Contains(titleLower, "term") {
			return
		}

		// Find metadata by looking inside the container. The live .event-date
		// markup stacks day/month spans across lines, so collapse whitespace.
		dateStr := normalizeSpace(sel.Find(".event-date, .date, .time, .field--name-field-event-date").First().Text())
		category := strings.TrimSpace(sel.Find(".event-category, .category, .type, .field--name-field-event-category").First().Text())

		// The container itself may be the <a> (live .event-item markup), so check
		// it before falling back to nested anchors.
		link := strings.TrimSpace(sel.AttrOr("href", ""))
		if link == "" {
			link, _ = s.Attr("href")
		}
		if link == "" {
			link, _ = s.Find("a").First().Attr("href")
		}
		if link == "" {
			link, _ = sel.Find("a").First().Attr("href")
		}

		if c.Verbose {
			logger.Log.Debug("vcc candidate", "title", title, "category", category, "date", dateStr)
		}

		if !c.isRelevant(title, category) {
			if c.Verbose {
				logger.Log.Debug("vcc skip irrelevant", "title", title)
			}
			return
		}

		// Normalize link
		if link != "" && !strings.HasPrefix(link, "http") {
			link = "https://www.vancouverconventioncentre.com" + link
		}

		eventID := c.slugify(title + " " + dateStr)
		eventHash := fmt.Sprintf("%x", sha256.Sum256([]byte("vcc_events|"+eventID)))
		project := collector.RawProject{
			Source:      c.Name(),
			ExternalID:  eventID,
			Title:       title,
			Description: fmt.Sprintf("Category: %s | Date: %s", category, dateStr),
			SourceURL:   link,
			RawData:     body,
			RawType:     "text/html",
			Hash:        eventHash,
		}
		projects = append(projects, project)
	})

	if c.Verbose {
		logger.Log.Info("vcc collection complete", "count", len(projects))
	}
	if len(projects) == 0 && c.Verbose {
		logger.Log.Debug("vcc no projects found")
	}

	return projects, nil
}

func (c *VCCCollector) isRelevant(title, category string) bool {
	title = strings.ToLower(title)
	category = strings.ToLower(category)

	keep := []string{"conference", "congress", "summit", "symposium", "trade show", "expo", "convention", "meeting", "forum"}
	drop := []string{"consumer show", "home show", "auto show", "art fair", "comedy", "concert", "wedding", "graduation", "public show"}

	for _, d := range drop {
		if strings.Contains(title, d) || strings.Contains(category, d) {
			return false
		}
	}

	for _, k := range keep {
		if strings.Contains(title, k) || strings.Contains(category, k) {
			return true
		}
	}

	// If no explicit keep/drop, default to true for now to let Claude decide
	return true
}

// normalizeSpace collapses any run of whitespace (including newlines from
// stacked markup) into single spaces and trims the result.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (c *VCCCollector) slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Simple slugify for now
	return s
}
