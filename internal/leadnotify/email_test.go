package leadnotify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

func TestSendLeads_noRecipientsIsNoop(t *testing.T) {
	n := NewEmailNotifier("", "") // empty key must not matter when there's nothing to send
	leads := []storage.Lead{{Title: "Test"}}
	if err := n.SendLeads(context.Background(), nil, leads); err != nil {
		t.Fatalf("expected nil for no recipients, got %v", err)
	}
}

func TestSendLeads_noLeadsIsNoop(t *testing.T) {
	n := NewEmailNotifier("", "")
	if err := n.SendLeads(context.Background(), []string{"a@example.com"}, nil); err != nil {
		t.Fatalf("expected nil for no leads, got %v", err)
	}
}

func TestSendLeads_missingAPIKeyErrors(t *testing.T) {
	n := NewEmailNotifier("", "")
	leads := []storage.Lead{{Title: "Test"}}
	if err := n.SendLeads(context.Background(), []string{"a@example.com"}, leads); err == nil {
		t.Fatal("expected error when RESEND_API_KEY is unset but a send is required")
	}
}

func TestGenerateDigestHTMLWithAnalytics(t *testing.T) {
	html, err := generateDigestHTMLWithAnalytics(
		[]storage.Lead{{Title: "Bridge project", Source: "announcements", PriorityScore: 9}},
		[]storage.SourceAttribution{{Source: "announcements", Leads: 5, Claimed: 2, Won: 1, HitRate: 60}},
		[]storage.DemandBucket{{
			WeekStart:         time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
			Source:            "announcements",
			Leads:             2,
			EstimatedCrewSize: 50,
		}},
	)
	if err != nil {
		t.Fatalf("generateDigestHTMLWithAnalytics: %v", err)
	}

	for _, want := range []string{
		"Source Attribution",
		"announcements",
		"60.0%",
		"Demand Density By Week",
		"2026-07-06",
		"50",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("digest HTML missing %q", want)
		}
	}
}
