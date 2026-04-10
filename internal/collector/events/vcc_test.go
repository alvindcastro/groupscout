package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVCCCollector_Collect(t *testing.T) {
	mockHTML := `
		<html>
			<body>
				<div class="views-row">
					<div class="event-title"><h3>BCTech Summit 2026</h3></div>
					<div class="event-date">March 10-12, 2026</div>
					<div class="event-category">Conference</div>
					<a href="/events/bctech-summit">View Event</a>
				</div>
				<div class="views-row">
					<div class="event-title"><h3>Vancouver International Auto Show</h3></div>
					<div class="event-date">March 20-25, 2026</div>
					<div class="event-category">Consumer Show</div>
					<a href="/events/auto-show">View Event</a>
				</div>
				<div class="views-row">
					<div class="event-title"><h3>Global Mining Symposium</h3></div>
					<div class="event-date">April 5, 2026</div>
					<div class="event-category">Symposium</div>
					<a href="https://example.com/mining">External Link</a>
				</div>
			</body>
		</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	c := NewVCCCollector(server.URL)
	projects, err := c.Collect(context.Background())

	assert.NoError(t, err)
	assert.Len(t, projects, 2)

	assert.Equal(t, "BCTech Summit 2026", projects[0].Title)
	assert.Equal(t, "Category: Conference | Date: March 10-12, 2026", projects[0].Description)
	assert.Equal(t, "https://www.vancouverconventioncentre.com/events/bctech-summit", projects[0].SourceURL)

	assert.Equal(t, "Global Mining Symposium", projects[1].Title)
	assert.Equal(t, "https://example.com/mining", projects[1].SourceURL)
}

func TestVCCCollector_IsRelevant(t *testing.T) {
	c := NewVCCCollector("")
	tests := []struct {
		title    string
		category string
		expected bool
	}{
		{"BCTech Summit", "Conference", true},
		{"Mining Symposium", "Forum", true},
		{"Auto Show", "Consumer Show", false},
		{"Vancouver Wedding Fair", "Exhibition", false},
		{"Medical Congress", "Meeting", true},
		{"Random Event", "Random Category", true}, // Default true
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.expected, c.isRelevant(tt.title, tt.category))
		})
	}
}
