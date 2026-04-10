package aviation

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type NavCanadaClient struct {
	client *http.Client
	url    string
}

func NewNavCanadaClient() *NavCanadaClient {
	return &NavCanadaClient{
		client: &http.Client{Timeout: 10 * time.Second},
		url:    "https://www.navcanada.ca/en/flight-planning/notam-portal.aspx", // Placeholder for actual API/portal
	}
}

func (c *NavCanadaClient) FetchGroundStop(ctx context.Context, airportCode string) (bool, error) {
	// Example implementation: real world would probably use a POST with airportCode
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s?airport=%s", c.url, airportCode), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return parseNOTAMs(resp.Body)
}

func parseNOTAMs(body io.Reader) (bool, error) {
	// In reality we'd parse the HTML with goquery, but for now we look for the indicator strings
	// in the entire body as a simple heuristic for Ground Stop detection.
	all, err := io.ReadAll(body)
	if err != nil {
		return false, err
	}

	content := strings.ToUpper(string(all))
	if strings.Contains(content, "GND STOP") || strings.Contains(content, "GROUND STOP") {
		return true, nil
	}

	return false, nil
}
