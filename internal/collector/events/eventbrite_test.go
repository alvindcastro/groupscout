package events

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/stretchr/testify/assert"
)

func TestEventbriteCollector_IsRelevant(t *testing.T) {
	c := &EventbriteCollector{}

	tests := []struct {
		name     string
		title    string
		metadata string
		expected bool
	}{
		{
			"Relevant conference",
			"Global Construction Summit 2026",
			"A gathering of industry professionals to discuss future infrastructure.",
			true,
		},
		{
			"Relevant trade show",
			"Build BC Trade Show",
			"Showcase for building materials and construction technology.",
			true,
		},
		{
			"Relevant corporate",
			"Real Estate Investment Forum",
			"Networking event for business leaders and corporate investors.",
			true,
		},
		{
			"Irrelevant party",
			"Friday Night Dance Party",
			"Join us for a night of electronic music and clubbing.",
			false,
		},
		{
			"Irrelevant hobby",
			"Summer Craft Fair",
			"A market for handmade gifts and local crafts hobbyists.",
			false,
		},
		{
			"Mixed content with negative keyword",
			"Real Estate Business Networking and Rave",
			"A networking session followed by a late night rave.",
			false,
		},
		{
			"Generic workshop",
			"Learn to Bake Bread",
			"A hands-on workshop for home bakers.",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, c.isRelevant(tt.title, tt.metadata))
		})
	}
}

func TestEventbriteCollector_Parse(t *testing.T) {
	html := `
	<html>
		<body>
			<section class="event-card-details">
				<h3 class="event-card-title">Professional Networking Summit</h3>
				<a class="event-card-link" href="https://eventbrite.ca/e/summit-123">Details</a>
				<div class="metadata">Professional gathering in Vancouver.</div>
			</section>
			<div class="DiscoverHorizontalEventCard-module__cardContent___1f9Xv">
				<h2>Construction Industry Conference</h2>
				<a href="https://eventbrite.ca/e/conf-456">Link</a>
				<span>Industry news and construction trends.</span>
			</div>
			<div class="event-card">
				<h3 class="event-card-link">Yoga in the Park</h3>
				<a href="https://eventbrite.ca/e/yoga-789">Yoga link</a>
				<span>Relaxing yoga session.</span>
			</div>
		</body>
	</html>
	`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	c := &EventbriteCollector{Verbose: true}

	var projects []collector.RawProject
	doc.Find("section.event-card-details, div.DiscoverHorizontalEventCard-module__cardContent___1f9Xv, div.event-card").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("h3, h2, a.event-card-link").First().Text())
		link, _ := s.Find("a.event-card-link, a").First().Attr("href")
		metadata := strings.TrimSpace(s.Text())

		if c.isRelevant(title, metadata) {
			p := collector.RawProject{
				Title:       title,
				SourceURL:   link,
				Description: metadata,
			}
			projects = append(projects, p)
		}
	})

	assert.Len(t, projects, 2)
	assert.Equal(t, "Professional Networking Summit", projects[0].Title)
	assert.Equal(t, "Construction Industry Conference", projects[1].Title)
}
