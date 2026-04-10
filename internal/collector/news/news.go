package news

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/mmcdole/gofeed"
)

// NewsCollector fetches and parses news from an RSS feed (e.g., Google News).
type NewsCollector struct {
	RSSURL  string
	Verbose bool
}

func NewNewsCollector(rssURL string) *NewsCollector {
	return &NewsCollector{
		RSSURL: rssURL,
	}
}

func (c *NewsCollector) Name() string {
	return "google_news"
}

func (c *NewsCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	if c.Verbose {
		logger.Log.Info("collecting news started")
	}
	if c.RSSURL == "" {
		if c.Verbose {
			logger.Log.Warn("news RSSURL is empty")
		}
		return nil, nil
	}

	fp := gofeed.NewParser()
	var allProjects []collector.RawProject

	urls := strings.Split(c.RSSURL, ",")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}

		if c.Verbose {
			logger.Log.Info("fetching news RSS", "url", url)
		}
		feed, err := fp.ParseURLWithContext(url, ctx)
		if err != nil {
			if c.Verbose {
				logger.Log.Error("failed to parse news RSS feed", "url", url, "error", err)
			}
			continue
		}

		for _, item := range feed.Items {
			if !c.isRelevant(item) {
				continue
			}

			project := collector.RawProject{
				Source:      "google_news",
				ExternalID:  item.GUID,
				Title:       item.Title,
				Description: item.Description,
				SourceURL:   item.Link,
			}

			if item.PublishedParsed != nil {
				project.IssuedAt = *item.PublishedParsed
			}

			// Generate hash based on source and stable external ID (link or GUID)
			id := item.Link
			if item.GUID != "" {
				id = item.GUID
			}
			h := sha256.New()
			h.Write([]byte(fmt.Sprintf("google_news:%s", id)))
			project.Hash = fmt.Sprintf("%x", h.Sum(nil))

			allProjects = append(allProjects, project)
		}
	}

	if c.Verbose {
		logger.Log.Info("news collection complete", "count", len(allProjects))
	}
	return allProjects, nil
}

// isRelevant filters for construction/infrastructure news.
func (c *NewsCollector) isRelevant(item *gofeed.Item) bool {
	content := strings.ToLower(item.Title + " " + item.Description)

	// Positive keywords
	keywords := []string{
		"construction", "infrastructure", "contract awarded", "expansion",
		"groundbreaking", "permit", "development", "industrial", "warehouse",
		"building", "residential", "housing", "project",
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

	// Negative keywords to filter out non-construction news (e.g. sports, political gossip)
	negativeKeywords := []string{
		"score", "game", "match", "win", "loss", "election", "campaign",
	}

	for _, kw := range negativeKeywords {
		if strings.Contains(content, kw) {
			return false
		}
	}

	return true
}
