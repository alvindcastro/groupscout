package enrichment

import (
	"context"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/google/uuid"
)

// ── toLeadRecord ──────────────────────────────────────────────────────────────

func TestToLeadRecord_fieldMapping(t *testing.T) {
	issued := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	p := collector.RawProject{
		Source:   "richmond_permits",
		Title:    "Hotel — 8640 Alexandra Road",
		Location: "8640 Alexandra Road",
		Value:    300_000,
		IssuedAt: issued,
		Metadata: map[string]any{
			"applicant":  "Studio Senbel Architecture Inc (604)605-6995",
			"contractor": "Safara Cladding Inc (416)875-1770",
		},
	}
	e := &EnrichedLead{
		GeneralContractor:       "Safara Cladding Inc",
		ProjectType:             "commercial",
		EstimatedCrewSize:       15,
		EstimatedDurationMonths: 2,
		OutOfTownCrewLikely:     false,
		PriorityScore:           4,
		PriorityReason:          "Small alteration — local crew likely",
		SuggestedOutreachTiming: "Low priority — monitor for future phases",
		Notes:                   "Cladding alteration only.",
	}

	lead := toLeadRecord(p, e, "")

	// RawProject fields
	if lead.Source != "richmond_permits" {
		t.Errorf("Source = %q, want %q", lead.Source, "richmond_permits")
	}
	if lead.Title != p.Title {
		t.Errorf("Title = %q, want %q", lead.Title, p.Title)
	}
	if lead.Location != p.Location {
		t.Errorf("Location = %q, want %q", lead.Location, p.Location)
	}
	if lead.ProjectValue != p.Value {
		t.Errorf("ProjectValue = %d, want %d", lead.ProjectValue, p.Value)
	}

	// Raw applicant/contractor from Metadata (phone numbers preserved)
	if lead.Applicant != "Studio Senbel Architecture Inc (604)605-6995" {
		t.Errorf("Applicant = %q, want phone number in string", lead.Applicant)
	}
	if lead.Contractor != "Safara Cladding Inc (416)875-1770" {
		t.Errorf("Contractor = %q, want phone number in string", lead.Contractor)
	}

	// EnrichedLead fields
	if lead.GeneralContractor != e.GeneralContractor {
		t.Errorf("GeneralContractor = %q, want %q", lead.GeneralContractor, e.GeneralContractor)
	}
	if lead.PriorityScore != 4 {
		t.Errorf("PriorityScore = %d, want 4", lead.PriorityScore)
	}
	if lead.OutOfTownCrewLikely != false {
		t.Errorf("OutOfTownCrewLikely = %v, want false", lead.OutOfTownCrewLikely)
	}
}

func TestToLeadRecord_missingMetadataKeys(t *testing.T) {
	p := collector.RawProject{
		Source:   "richmond_permits",
		Metadata: map[string]any{}, // no applicant or contractor keys
	}
	lead := toLeadRecord(p, &EnrichedLead{}, "")
	if lead.Applicant != "" {
		t.Errorf("Applicant should be empty when key absent, got %q", lead.Applicant)
	}
	if lead.Contractor != "" {
		t.Errorf("Contractor should be empty when key absent, got %q", lead.Contractor)
	}
}

func TestToLeadRecord_nilMetadata(t *testing.T) {
	p := collector.RawProject{
		Source:   "richmond_permits",
		Metadata: nil,
	}
	// Should not panic when Metadata is nil
	lead := toLeadRecord(p, &EnrichedLead{}, "")
	if lead.Applicant != "" || lead.Contractor != "" {
		t.Errorf("expected empty strings for nil Metadata, got applicant=%q contractor=%q",
			lead.Applicant, lead.Contractor)
	}
}

func TestToLeadRecord_rawInputIDPropagated(t *testing.T) {
	rawID := "550e8400-e29b-41d4-a716-446655440000"
	p := collector.RawProject{Source: "richmond_permits"}
	lead := toLeadRecord(p, &EnrichedLead{}, rawID)
	if lead.RawInputID != rawID {
		t.Errorf("RawInputID = %q, want %q", lead.RawInputID, rawID)
	}
}

// ── processProject ────────────────────────────────────────────────────────────

type mockAuditStore struct {
	storage.AuditStore
	existsByHash func(hash string) (bool, error)
	store        func(raw storage.RawInput) (uuid.UUID, error)
}

func (m *mockAuditStore) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	return m.existsByHash(hash)
}

func (m *mockAuditStore) Store(ctx context.Context, raw storage.RawInput) (uuid.UUID, error) {
	return m.store(raw)
}

type mockRawStore struct {
	storage.RawProjectStore
	existsByHash func(hash string) (bool, error)
	insert       func(p *collector.RawProject) error
}

func (m *mockRawStore) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	return m.existsByHash(hash)
}

func (m *mockRawStore) Insert(ctx context.Context, p *collector.RawProject) error {
	return m.insert(p)
}

type mockLeadStore struct {
	storage.LeadStore
	insert func(l *storage.Lead) error
}

func (m *mockLeadStore) Insert(ctx context.Context, l *storage.Lead) error {
	return m.insert(l)
}

type mockAI struct {
	EnricherAI
	enrich func(p collector.RawProject) (*EnrichedLead, error)
}

func (m *mockAI) Enrich(ctx context.Context, p collector.RawProject) (*EnrichedLead, error) {
	return m.enrich(p)
}

func TestEnricher_processProject_storesAllAuditFields(t *testing.T) {
	var capturedRaw storage.RawInput
	audit := &mockAuditStore{
		existsByHash: func(hash string) (bool, error) { return false, nil },
		store: func(raw storage.RawInput) (uuid.UUID, error) {
			capturedRaw = raw
			return uuid.New(), nil
		},
	}
	raw := &mockRawStore{
		existsByHash: func(hash string) (bool, error) { return false, nil },
		insert:       func(p *collector.RawProject) error { return nil },
	}
	leads := &mockLeadStore{
		insert: func(l *storage.Lead) error { return nil },
	}
	ai := &mockAI{
		enrich: func(p collector.RawProject) (*EnrichedLead, error) {
			return &EnrichedLead{PriorityScore: 7}, nil
		},
	}

	e := NewEnricher(nil, raw, audit, leads, ai, NewScorer(5), 0, nil, nil, false, false)

	payload := []byte("%PDF-1.4 test")
	expectedHash := storage.HashPayload(payload)

	p := collector.RawProject{
		Hash:      "test-hash",
		Source:    "richmond_permits",
		RawType:   "application/pdf",
		RawData:   payload,
		SourceURL: "https://richmond.ca/permits/test.pdf",
	}

	if _, err := e.processProject(context.Background(), p); err != nil {
		t.Fatalf("processProject: %v", err)
	}

	if capturedRaw.Hash != expectedHash {
		t.Errorf("Hash = %q, want %q", capturedRaw.Hash, expectedHash)
	}
	if capturedRaw.PayloadType != "application/pdf" {
		t.Errorf("PayloadType = %q, want %q", capturedRaw.PayloadType, "application/pdf")
	}
	if string(capturedRaw.Payload) != "%PDF-1.4 test" {
		t.Errorf("Payload = %q, want PDF bytes", string(capturedRaw.Payload))
	}
	if capturedRaw.SourceURL != "https://richmond.ca/permits/test.pdf" {
		t.Errorf("SourceURL = %q, want %q", capturedRaw.SourceURL, "https://richmond.ca/permits/test.pdf")
	}
	if capturedRaw.CollectorName != "richmond_permits" {
		t.Errorf("CollectorName = %q, want %q", capturedRaw.CollectorName, "richmond_permits")
	}
}

func TestEnricher_processProject_dedupCheck(t *testing.T) {
	raw := &mockRawStore{
		existsByHash: func(hash string) (bool, error) {
			if hash == "seen-hash" {
				return true, nil
			}
			return false, nil
		},
	}
	e := &Enricher{
		rawStore: raw,
		Verbose:  true,
	}

	p := collector.RawProject{Hash: "seen-hash"}
	inserted, err := e.processProject(context.Background(), p)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inserted {
		t.Errorf("expected inserted=false for seen-hash")
	}
}

func TestEnricher_processProject_callsStoreAndLinksID(t *testing.T) {
	rawID := uuid.New()
	payload := []byte("test payload")
	expectedHash := storage.HashPayload(payload)

	audit := &mockAuditStore{
		store: func(raw storage.RawInput) (uuid.UUID, error) {
			if raw.Hash != expectedHash {
				t.Errorf("expected hash %q, got %q", expectedHash, raw.Hash)
			}
			return rawID, nil
		},
	}
	raw := &mockRawStore{
		existsByHash: func(hash string) (bool, error) { return false, nil },
		insert:       func(p *collector.RawProject) error { return nil },
	}

	var insertedLead *storage.Lead
	leads := &mockLeadStore{
		insert: func(l *storage.Lead) error {
			insertedLead = l
			return nil
		},
	}

	ai := &mockAI{
		enrich: func(p collector.RawProject) (*EnrichedLead, error) {
			return &EnrichedLead{PriorityScore: 9}, nil
		},
	}

	e := NewEnricher(nil, raw, audit, leads, ai, NewScorer(5), 0, nil, nil, false, false)

	p := collector.RawProject{
		Hash:       "new-hash",
		ExternalID: "EXT-123",
		Source:     "test-source",
		Title:      "Test Project",
		RawData:    []byte("test payload"),
	}

	inserted, err := e.processProject(context.Background(), p)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inserted {
		t.Fatalf("expected inserted=true")
	}

	if insertedLead == nil {
		t.Fatal("lead was not inserted")
	}
	if insertedLead.RawInputID != rawID.String() {
		t.Errorf("RawInputID = %q, want %q", insertedLead.RawInputID, rawID.String())
	}
}

func TestEnricher_processProject_linksIDToSkippedLead(t *testing.T) {
	rawID := uuid.New()
	audit := &mockAuditStore{
		store: func(raw storage.RawInput) (uuid.UUID, error) { return rawID, nil },
	}
	raw := &mockRawStore{
		existsByHash: func(hash string) (bool, error) { return false, nil },
		insert:       func(p *collector.RawProject) error { return nil },
	}

	var insertedLead *storage.Lead
	leads := &mockLeadStore{
		insert: func(l *storage.Lead) error {
			insertedLead = l
			return nil
		},
	}

	e := NewEnricher(nil, raw, audit, leads, nil, NewScorer(50), 0, nil, nil, false, false)

	p := collector.RawProject{
		Hash:       "skipped-hash",
		ExternalID: "EXT-456",
		Source:     "test-source",
		Title:      "Low Score Project",
		Value:      100, // low score
	}

	inserted, err := e.processProject(context.Background(), p)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inserted {
		t.Fatalf("expected inserted=true (even for skipped leads)")
	}

	if insertedLead == nil {
		t.Fatal("skipped lead was not inserted")
	}
	if insertedLead.Status != "skipped" {
		t.Errorf("status = %q, want %q", insertedLead.Status, "skipped")
	}
	if insertedLead.RawInputID != rawID.String() {
		t.Errorf("RawInputID = %q, want %q", insertedLead.RawInputID, rawID.String())
	}
}
