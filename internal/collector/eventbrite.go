package collector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// EventbriteCollector scrapes Eventbrite for professional events in Vancouver.
type EventbriteCollector struct {
	URL     string
	Verbose bool
}

func NewEventbriteCollector(url string) *EventbriteCollector {
	return &EventbriteCollector{
		URL: url,
	}
}

func (c *EventbriteCollector) Name() string {
	return "eventbrite"
}

func (c *EventbriteCollector) Collect(ctx context.Context) ([]RawProject, error) {
	if c.Verbose {
		log.Printf("[Eventbrite] Collect started from %s", c.URL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add user-agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var projects []RawProject

	// Eventbrite structure often uses 'section.event-card-details' or similar.
	// We'll target a broad set of common classes for cards and links.
	doc.Find("section.event-card-details, div.DiscoverHorizontalEventCard-module__cardContent___1f9Xv, div.event-card").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("h3, h2, a.event-card-link").First().Text())
		link, _ := s.Find("a.event-card-link, a").First().Attr("href")

		// Metadata often in sub-text or sibling divs
		metadata := strings.TrimSpace(s.Text())

		if title == "" || link == "" {
			return
		}

		// Basic relevance check: skip consumer/hobby stuff
		if !c.isRelevant(title, metadata) {
			return
		}

		p := RawProject{
			Source:      "eventbrite",
			ExternalID:  link, // Use link as ID
			Title:       title,
			Description: metadata,
			SourceURL:   link,
			IssuedAt:    time.Now(), // Eventbrite doesn't always have clear "published" dates in list view
		}

		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("eventbrite:%s", link)))
		p.Hash = fmt.Sprintf("%x", h.Sum(nil))

		projects = append(projects, p)
	})

	if c.Verbose {
		log.Printf("[Eventbrite] Collected %d potential events", len(projects))
	}

	return projects, nil
}

func (c *EventbriteCollector) isRelevant(title, metadata string) bool {
	content := strings.ToLower(title + " " + metadata)

	// Positive keywords for professional/lodging-heavy events
	keywords := []string{
		"conference", "summit", "forum", "convention", "trade show", "symposium",
		"professional", "industry", "business", "networking", "workshop", "seminar",
		"corporate", "training", "association", "government", "tech", "summit",
	}

	relevant := false
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			relevant = true
			break
		}
	}

	if !relevant {
		return false
	}

	// Negative keywords to skip consumer/party/hobby events
	negatives := []string{
		"party", "concert", "dance", "nightlife", "yoga", "fitness", "dating",
		"hobby", "craft", "market", "sale", "festival", "clubbing", "rave",
	}

	for _, kw := range negatives {
		if strings.Contains(content, kw) {
			return false
		}
	}

	return true
}
