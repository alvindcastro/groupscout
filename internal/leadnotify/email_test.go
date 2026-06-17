package leadnotify

import (
	"context"
	"testing"

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
