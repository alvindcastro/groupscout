package events

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var vccMockHTML = `
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

func TestVCCCollector_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(vccMockHTML))
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

func TestVCCCollector_PerEventHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(vccMockHTML))
	}))
	defer server.Close()

	c := NewVCCCollector(server.URL)
	projects, err := c.Collect(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 2)

	// Each event must have a distinct, non-empty hash so the dedup system
	// processes all events from the same page, not just the first.
	assert.NotEmpty(t, projects[0].Hash)
	assert.NotEmpty(t, projects[1].Hash)
	assert.NotEqual(t, projects[0].Hash, projects[1].Hash, "each VCC event must have a unique dedup hash")
}

func TestVCCCollector_404_SourceDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewVCCCollector(server.URL)
	_, err := c.Collect(context.Background())

	require.Error(t, err)
	var driftErr *collector.SourceDriftError
	assert.True(t, errors.As(err, &driftErr), "404 should return SourceDriftError, not generic error")
	assert.Equal(t, http.StatusNotFound, driftErr.StatusCode)
	assert.Equal(t, server.URL, driftErr.URL)
}

func TestVCCCollector_410_SourceDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	c := NewVCCCollector(server.URL)
	_, err := c.Collect(context.Background())

	require.Error(t, err)
	var driftErr *collector.SourceDriftError
	assert.True(t, errors.As(err, &driftErr), "410 should return SourceDriftError")
	assert.Equal(t, http.StatusGone, driftErr.StatusCode)
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
