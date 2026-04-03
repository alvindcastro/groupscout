package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

func (c *VCCCollector) Collect(ctx context.Context) ([]RawProject, error) {
	if c.Verbose {
		log.Printf("[vcc] fetching events from: %s", c.url)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if c.Verbose {
		log.Printf("[vcc] HTTP 200 OK. Content-Type: %s", resp.Header.Get("Content-Type"))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var projects []RawProject
	// The VCC events page typically has event cards.
	// We'll need to inspect the actual markup to be sure, but let's start with a likely structure.
	// Based on common patterns and PHASES.md, we want to extract:
	// name, start date, end date, category tag.

	doc.Find("h2, h3, h4").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" || len(title) > 100 || strings.Contains(strings.ToLower(title), "search") {
			return
		}

		// Find the closest parent that might contain date/category
		// Usually VCC titles are inside a container with other info
		parent := s.Parent()
		dateStr := strings.TrimSpace(parent.Find(".event-date, .date, .time").First().Text())
		category := strings.TrimSpace(parent.Find(".event-category, .category, .type").First().Text())
		link, _ := parent.Find("a").First().Attr("href")
		if link == "" {
			link, _ = s.Find("a").First().Attr("href")
		}

		if !c.isRelevant(title, category) {
			return
		}

		// Normalize link
		if link != "" && !strings.HasPrefix(link, "http") {
			link = "https://www.vancouverconventioncentre.com" + link
		}

		project := RawProject{
			Source:      c.Name(),
			ExternalID:  c.slugify(title + " " + dateStr),
			Title:       title,
			Description: fmt.Sprintf("Category: %s | Date: %s", category, dateStr),
			SourceURL:   link,
		}
		// In actual production, hashing would happen in storage/raw.go if using the repository pattern
		// but since collectors return RawProject, we'll let the pipeline handle hashing if needed.
		// However, other collectors seem to set the Hash themselves.
		// Actually, let's see how others do it.
		projects = append(projects, project)
	})

	if c.Verbose {
		log.Printf("[vcc] found %d relevant events", len(projects))
	}
	if len(projects) == 0 && c.Verbose {
		log.Println("[vcc] debug: no projects found.")
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

func (c *VCCCollector) slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Simple slugify for now
	return s
}
