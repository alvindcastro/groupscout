package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnnouncementsCollector_parseBCIB(t *testing.T) {
	// Mirrors bcib.ca/projects/: project cards, map markers, and the nav and
	// audience-card headings that must not become projects.
	html := `<html><body>
		<nav><h3 class="body-text _uppercase _20">How do I</h3><h3 class="body-text _uppercase _20">More</h3></nav>
		<div class="card-column">
			<h3 class="title _20 title-text _medium _dark">Pattullo Bridge Replacement Project</h3>
			<p>A new bridge connecting Surrey and New Westminster.</p>
		</div>
		<div class="feature-cards-column"><h3 class="title-text _32 _medium">Contractor</h3></div>
		<div class="acf-map"><div class="marker">
			<h3>Broadway Subway Project</h3>
			<p>Extending the Millennium Line.</p>
		</div></div>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewAnnouncementsCollector()
	c.Sources = []AnnouncementSource{
		{Name: "BCIB", URL: server.URL, Type: "bcib"},
	}

	projects, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	if projects[0].Title != "Pattullo Bridge Replacement Project" {
		t.Errorf("Expected title Pattullo Bridge Replacement Project, got %s", projects[0].Title)
	}
}

func TestAnnouncementsCollector_parseTransLink(t *testing.T) {
	html := `<html><body>
		<h4>Maintenance and Upgrade Program</h4>
		<p>Keeping the system running.</p>
		<h4>Rapid Transit Projects</h4>
		<p>SkyTrain expansions.</p>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewAnnouncementsCollector()
	c.Sources = []AnnouncementSource{
		{Name: "TransLink", URL: server.URL, Type: "translink"},
	}

	projects, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	if projects[0].Title != "Maintenance and Upgrade Program" {
		t.Errorf("Expected title Maintenance and Upgrade Program, got %s", projects[0].Title)
	}
}

func TestAnnouncementsCollector_parseYVR(t *testing.T) {
	html := `<html><body>
		<h3>North Runway Modernization Program</h3>
		<h3>2026 South Airfield Maintenance</h3>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewAnnouncementsCollector()
	c.Sources = []AnnouncementSource{
		{Name: "YVR", URL: server.URL, Type: "yvr"},
	}

	projects, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	if projects[0].Title != "North Runway Modernization Program" {
		t.Errorf("Expected title North Runway Modernization Program, got %s", projects[0].Title)
	}
}
