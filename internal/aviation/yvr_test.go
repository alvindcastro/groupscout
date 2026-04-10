package aviation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseYVRFlightStatus(t *testing.T) {
	fixture := `
	<div class="flight-status">
		<table>
			<tr class="flight-row"><td>AC101</td><td>Cancelled</td></tr>
			<tr class="flight-row"><td>WS202</td><td>On Time</td></tr>
			<tr class="flight-row"><td>AC103</td><td>Cancelled</td></tr>
			<tr class="flight-row"><td>UA404</td><td>On Time</td></tr>
			<tr class="flight-row"><td>WS505</td><td>Cancelled</td></tr>
			<tr class="flight-row"><td>AC106</td><td>Delayed</td></tr>
			<tr class="flight-row"><td>AC107</td><td>On Time</td></tr>
			<tr class="flight-row"><td>AC108</td><td>On Time</td></tr>
			<tr class="flight-row"><td>AC109</td><td>On Time</td></tr>
			<tr class="flight-row"><td>AC110</td><td>On Time</td></tr>
		</table>
	</div>
	`
	status, err := parseYVRFlightStatus(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if status.CancelledCount != 3 {
		t.Errorf("expected 3 cancelled, got %d", status.CancelledCount)
	}
	if status.TotalDepartures != 10 {
		t.Errorf("expected 10 total, got %d", status.TotalDepartures)
	}
	if status.CancellationRate != 0.30 {
		t.Errorf("expected 0.30 rate, got %f", status.CancellationRate)
	}
}

func TestParseYVRFlightStatus_NoCancellations(t *testing.T) {
	fixture := `
	<table>
		<tr class="flight-row"><td>AC101</td><td>On Time</td></tr>
	</table>
	`
	status, err := parseYVRFlightStatus(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if status.CancellationRate != 0.0 {
		t.Errorf("expected 0.0 rate, got %f", status.CancellationRate)
	}
}

func TestParseYVRFlightStatus_MalformedHTML(t *testing.T) {
	fixture := `<div>not a table</div>`
	status, err := parseYVRFlightStatus(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if status.TotalDepartures != 0 {
		t.Errorf("expected 0 total, got %d", status.TotalDepartures)
	}
}

func TestYVRScraper_FetchStatus(t *testing.T) {
	fixture := `<table><tr class="flight-row"><td>AC101</td><td>Cancelled</td></tr></table>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fixture))
	}))
	defer server.Close()

	scraper := &YVRScraper{
		client: server.Client(),
		url:    server.URL,
	}

	status, err := scraper.FetchStatus(context.Background())
	if err != nil {
		t.Fatalf("FetchStatus failed: %v", err)
	}

	if status.CancelledCount != 1 {
		t.Errorf("expected 1 cancelled, got %d", status.CancelledCount)
	}
}
