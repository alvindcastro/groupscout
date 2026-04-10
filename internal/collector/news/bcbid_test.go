package news

import (
	"context"
	"testing"
	"time"
)

func TestBCBidCollector_Collect(t *testing.T) {
	col := NewBCBidCollector(nil)

	t.Run("Empty input", func(t *testing.T) {
		ctx := context.Background()
		projects, err := col.Collect(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projects) != 0 {
			t.Errorf("expected 0 projects, got %d", len(projects))
		}
	})

	t.Run("JSON input array", func(t *testing.T) {
		raw := `[
			{
				"opportunity_id": "OR-123",
				"title": "Bridge Construction",
				"value": "$1,500,000.00",
				"award_date": "2026-04-01",
				"location": "Richmond, BC"
			},
			{
				"opportunity_id": "OR-456",
				"title": "Road Paving",
				"value": 500000,
				"award_date": "Apr 02, 2026"
			}
		]`
		ctx := context.WithValue(context.Background(), "bcbid_raw_input", raw)
		projects, err := col.Collect(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 2 {
			t.Errorf("expected 2 projects, got %d", len(projects))
		}

		if projects[0].ExternalID != "OR-123" || projects[0].Value != 1500000 {
			t.Errorf("project 0 mismatch: ID=%s, Value=%d", projects[0].ExternalID, projects[0].Value)
		}
		if projects[1].ExternalID != "OR-456" || projects[1].Value != 500000 {
			t.Errorf("project 1 mismatch: ID=%s, Value=%d", projects[1].ExternalID, projects[1].Value)
		}
	})

	t.Run("HTML input", func(t *testing.T) {
		raw := `<html><body>Award Notice for Project X</body></html>`
		ctx := context.WithValue(context.Background(), "bcbid_raw_input", raw)
		projects, err := col.Collect(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(projects))
		}
		if projects[0].Title != "BC Bid Award Notice (HTML)" {
			t.Errorf("expected HTML title, got %q", projects[0].Title)
		}
		if projects[0].RawData["raw_html"] != raw {
			t.Errorf("raw_html mismatch")
		}
	})
}

func TestBCBidCollector_ParseValue(t *testing.T) {
	col := NewBCBidCollector(nil)
	tests := []struct {
		input    any
		expected int64
	}{
		{"$1,234,567.89", 1234567},
		{"500,000", 500000},
		{float64(123.45), 123},
		{int64(1000), 1000},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := col.parseValue(tt.input)
		if got != tt.expected {
			t.Errorf("parseValue(%v) = %d; expected %d", tt.input, got, tt.expected)
		}
	}
}

func TestBCBidCollector_ParseDate(t *testing.T) {
	col := NewBCBidCollector(nil)
	t1, _ := time.Parse("2006-01-02", "2026-04-01")
	t2, _ := time.Parse(time.RFC1123, "Fri, 03 Apr 2026 20:37:58 GMT")

	tests := []struct {
		input    string
		expected time.Time
	}{
		{"2026-04-01", t1},
		{"Apr 01, 2026", t1},
		{"Fri, 03 Apr 2026 20:37:58 GMT", t2},
		{"invalid", time.Now()}, // Should return now
	}

	for _, tt := range tests {
		got := col.parseDate(tt.input)
		if tt.input == "invalid" {
			if time.Since(got) > time.Second {
				t.Errorf("parseDate(%q) did not return recent time", tt.input)
			}
		} else if !got.Equal(tt.expected) {
			t.Errorf("parseDate(%q) = %v; expected %v", tt.input, got, tt.expected)
		}
	}
}
