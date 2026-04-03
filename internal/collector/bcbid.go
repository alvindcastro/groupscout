package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/logger"
)

// BCBidCollector processes award notices from BC Bid and aggregators like CivicInfo BC.
type BCBidCollector struct {
	Verbose bool
	RSSURLs []string
}

// NewBCBidCollector returns a new BCBidCollector.
func NewBCBidCollector(rssURLs []string) *BCBidCollector {
	return &BCBidCollector{
		RSSURLs: rssURLs,
	}
}

// Name satisfies the Collector interface.
func (b *BCBidCollector) Name() string { return "bcbid" }

// Collect satisfies the Collector interface.
func (b *BCBidCollector) Collect(ctx context.Context) ([]RawProject, error) {
	var allProjects []RawProject

	// 1. Check for automated RSS feeds if enabled
	for _, url := range b.RSSURLs {
		if url == "" {
			continue
		}
		if b.Verbose {
			logger.Log.Info("fetching bcbid RSS", "url", url)
		}
		projects, err := b.fetchRSS(ctx, url)
		if err != nil {
			logger.Log.Error("failed to fetch bcbid RSS", "url", url, "error", err)
			continue
		}
		allProjects = append(allProjects, projects...)
	}

	// 2. Check for manual raw input in the context (legacy/override support)
	raw, ok := ctx.Value("bcbid_raw_input").(string)
	if ok && raw != "" {
		if b.Verbose {
			logger.Log.Info("processing manual bcbid raw input")
		}
		manualProjects, err := b.processRaw(raw)
		if err != nil {
			logger.Log.Error("failed to process manual bcbid input", "error", err)
		} else {
			allProjects = append(allProjects, manualProjects...)
		}
	}

	if len(allProjects) == 0 && b.Verbose {
		logger.Log.Info("no bcbid projects collected")
	}

	return allProjects, nil
}

func (b *BCBidCollector) fetchRSS(ctx context.Context, url string) ([]RawProject, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b.parseRSS(body, url)
}

type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Guid        string `xml:"guid"`
}

func (b *BCBidCollector) parseRSS(data []byte, sourceURL string) ([]RawProject, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	var projects []RawProject
	for _, item := range feed.Channel.Items {
		id := item.Guid
		if id == "" {
			id = item.Link
		}

		// Clean description (CivicInfo RSS descriptions often contain HTML)
		desc := item.Description
		// Basic HTML tag removal if needed, but for now keep it for Enricher

		projects = append(projects, RawProject{
			Source:      "bcbid",
			ExternalID:  id,
			Title:       item.Title,
			Description: desc,
			IssuedAt:    b.parseDate(item.PubDate),
			SourceURL:   item.Link,
			Hash:        b.hashRaw(fmt.Sprintf("%s|%s|%s", id, item.Title, item.Link)),
			RawData: map[string]any{
				"rss_source": sourceURL,
				"pub_date":   item.PubDate,
			},
		})
	}
	return projects, nil
}

func (b *BCBidCollector) processRaw(raw string) ([]RawProject, error) {
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
			return nil, fmt.Errorf("failed to parse input as JSON or HTML: %w", err)
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
	formats := []string{"2006-01-02", "Jan 02, 2006", "01/02/2006", time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC822Z}
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
