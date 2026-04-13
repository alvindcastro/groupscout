package news

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

// AnnouncementsCollector scrapes major infrastructure announcement pages.
type AnnouncementsCollector struct {
	Sources []AnnouncementSource
	Verbose bool
}

type AnnouncementSource struct {
	Name string
	URL  string
	Type string // "bcib", "translink", "yvr"
}

func NewAnnouncementsCollector() *AnnouncementsCollector {
	return &AnnouncementsCollector{
		Sources: []AnnouncementSource{
			{Name: "BCIB Projects", URL: "https://bcib.ca/projects/", Type: "bcib"},
			{Name: "TransLink Projects", URL: "https://www.translink.ca/plans-and-projects/projects", Type: "translink"},
			{Name: "YVR Major Projects", URL: "https://news.yvr.ca/?h=1&t=project", Type: "yvr"},
		},
	}
}

func (c *AnnouncementsCollector) Name() string {
	return "announcements"
}

func (c *AnnouncementsCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	var allProjects []collector.RawProject

	for _, src := range c.Sources {
		if c.Verbose {
			logger.Log.Info("scraping announcements source", "name", src.Name, "url", src.URL)
		}

		projects, err := c.scrapeSource(ctx, src)
		if err != nil {
			logger.Log.Error("failed to scrape announcements source", "name", src.Name, "error", err)
			continue
		}
		allProjects = append(allProjects, projects...)
	}

	return allProjects, nil
}

func (c *AnnouncementsCollector) scrapeSource(ctx context.Context, src AnnouncementSource) ([]collector.RawProject, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", src.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var projects []collector.RawProject
	switch src.Type {
	case "bcib":
		projects = c.parseBCIB(doc, src, body)
	case "translink":
		projects = c.parseTransLink(doc, src, body)
	case "yvr":
		projects = c.parseYVR(doc, src, body)
	}

	return projects, nil
}

func (c *AnnouncementsCollector) parseBCIB(doc *goquery.Document, src AnnouncementSource, rawData []byte) []collector.RawProject {
	var projects []collector.RawProject
	// BCIB uses h3 for project titles in the projects list
	doc.Find("h3").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}

		// Description is usually the next paragraph or surrounding div
		description := strings.TrimSpace(s.Next().Text())
		if description == "" {
			description = strings.TrimSpace(s.Parent().Text())
		}

		p := collector.RawProject{
			Source:      "announcements",
			ExternalID:  "bcib:" + title,
			Title:       title,
			Description: description,
			SourceURL:   src.URL,
			IssuedAt:    time.Now(),
			RawData:     rawData,
			RawType:     "text/html",
		}
		p.Hash = c.hash(p.Source, p.ExternalID)
		projects = append(projects, p)
	})
	return projects
}

func (c *AnnouncementsCollector) parseTransLink(doc *goquery.Document, src AnnouncementSource, rawData []byte) []collector.RawProject {
	var projects []collector.RawProject
	// TransLink uses h4 for project categories/titles
	doc.Find("h4").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}

		description := strings.TrimSpace(s.Next().Text())

		p := collector.RawProject{
			Source:      "announcements",
			ExternalID:  "translink:" + title,
			Title:       title,
			Description: description,
			SourceURL:   src.URL,
			IssuedAt:    time.Now(),
			RawData:     rawData,
			RawType:     "text/html",
		}
		p.Hash = c.hash(p.Source, p.ExternalID)
		projects = append(projects, p)
	})
	return projects
}

func (c *AnnouncementsCollector) parseYVR(doc *goquery.Document, src AnnouncementSource, rawData []byte) []collector.RawProject {
	var projects []collector.RawProject
	// YVR Newsroom uses h3 for major projects
	doc.Find("h3").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" || title == "Major Projects" {
			return
		}

		p := collector.RawProject{
			Source:      "announcements",
			ExternalID:  "yvr:" + title,
			Title:       title,
			Description: "Major project at YVR",
			SourceURL:   src.URL,
			IssuedAt:    time.Now(),
			RawData:     rawData,
			RawType:     "text/html",
		}
		p.Hash = c.hash(p.Source, p.ExternalID)
		projects = append(projects, p)
	})
	return projects
}

func (c *AnnouncementsCollector) hash(source, id string) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s", source, id)))
	return fmt.Sprintf("%x", h.Sum(nil))
}
