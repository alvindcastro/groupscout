package enrichment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/llm"
	"github.com/alvindcastro/groupscout/internal/storage"
)

// ── stripMarkdown ─────────────────────────────────────────────────────────────

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain json — unchanged", `{"key": "val"}`, `{"key": "val"}`},
		{"fenced json block", "```json\n{\"key\": \"val\"}\n```", `{"key": "val"}`},
		{"fenced no language tag", "```\n{\"key\": \"val\"}\n```", `{"key": "val"}`},
		{"leading/trailing whitespace", "  {\"key\": \"val\"}  ", `{"key": "val"}`},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		got := stripMarkdown(tt.input)
		if got != tt.want {
			t.Errorf("[%s] stripMarkdown(%q) = %q, want %q", tt.name, tt.input, got, tt.want)
		}
	}
}

// ── parseEnrichedLead ─────────────────────────────────────────────────────────

func TestParseEnrichedLead_typicalReply(t *testing.T) {
	text := `{"general_contractor":"Ledcor","project_type":"industrial","estimated_crew_size":120,
"estimated_duration_months":18,"out_of_town_crew_likely":true,"priority_score":8,
"priority_reason":"Large build near YVR","suggested_outreach_timing":"Now","notes":"GC from Alberta"}`
	got, err := parseEnrichedLead(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := EnrichedLead{
		GeneralContractor: "Ledcor", ProjectType: "industrial", EstimatedCrewSize: 120,
		EstimatedDurationMonths: 18, OutOfTownCrewLikely: true, PriorityScore: 8,
		PriorityReason: "Large build near YVR", SuggestedOutreachTiming: "Now", Notes: "GC from Alberta",
	}
	if *got != want {
		t.Errorf("got %+v\nwant %+v", *got, want)
	}
}

// Models other than Claude wrap the object in prose or fences and send
// numbers and booleans as strings; all of that should still parse.
func TestParseEnrichedLead_toleratesLooseReplies(t *testing.T) {
	text := "Here is the evaluation:\n```json\n" +
		`{"project_type":"TV Series","estimated_crew_size":"80-200","estimated_duration_months":"6",` +
		`"out_of_town_crew_likely":"true","priority_score":"7","notes":["PM: Jane","Richmond"]}` +
		"\n```\nLet me know if you need more."
	got, err := parseEnrichedLead(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PriorityScore != 7 || got.EstimatedCrewSize != 80 || got.EstimatedDurationMonths != 6 || !got.OutOfTownCrewLikely {
		t.Errorf("numbers/bools not coerced: %+v", *got)
	}
	if got.ProjectType != "tv_series" {
		t.Errorf("ProjectType = %q, want tv_series", got.ProjectType)
	}
	if got.GeneralContractor != "" || !strings.Contains(got.Notes, "PM: Jane") {
		t.Errorf("strings not read: %+v", *got)
	}
}

func TestParseEnrichedLead_defaultsAndClamps(t *testing.T) {
	got, err := parseEnrichedLead(`{"priority_score":6.6,"estimated_crew_size":-3}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PriorityScore != 7 || got.EstimatedCrewSize != 0 || got.ProjectType != "unknown" {
		t.Errorf("got %+v, want score 7, crew 0, type unknown", *got)
	}
}

func TestParseEnrichedLead_rejectsUnusableReplies(t *testing.T) {
	for name, text := range map[string]string{
		"not json":         "I cannot evaluate this permit.",
		"truncated":        `{"general_contractor":"Ledcor","priority_sc`,
		"no score":         `{"general_contractor":"Ledcor"}`,
		"score not number": `{"priority_score":"high"}`,
		"score too high":   `{"priority_score":42}`,
		"score zero":       `{"priority_score":0}`,
	} {
		if _, err := parseEnrichedLead(text); err == nil {
			t.Errorf("%s: expected an error for %q", name, text)
		}
	}
}

// ── LLMEnricher ───────────────────────────────────────────────────────────────

type fakeCompleter struct {
	reqs []llm.Request
	resp llm.Response
	err  error
}

func (f *fakeCompleter) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.reqs = append(f.reqs, req)
	return f.resp, f.err
}

func TestLLMEnricher_EnrichSendsNeutralRequestAndRecordsProvider(t *testing.T) {
	fc := &fakeCompleter{resp: llm.Response{Text: `{"priority_score":9,"project_type":"feature_film"}`, Provider: "openai", Model: "gpt-4.1-mini"}}
	lead, err := NewLLMEnricher(fc).Enrich(context.Background(), creativeBCFeatureFilm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lead.PriorityScore != 9 || lead.Provider != "openai" || lead.Model != "gpt-4.1-mini" {
		t.Errorf("lead = %+v", *lead)
	}

	req := fc.reqs[0]
	if req.UseCase != "enrich" || !req.JSON || req.System != systemPrompt || req.MaxTokens == 0 {
		t.Errorf("request = %+v", req)
	}
	if req.User != creativeBCPrompt(creativeBCFeatureFilm) {
		t.Error("Creative BC project did not get the Creative BC prompt")
	}
	if req.Validate == nil || req.Validate(`{"priority_score":0}`) == nil || req.Validate(`{"priority_score":5}`) != nil {
		t.Error("Validate must reject a reply without a 1-10 score and accept one with it")
	}
}

func TestLLMEnricher_EnrichReturnsChainError(t *testing.T) {
	chainErr := &llm.ExhaustedError{Errors: []error{&llm.Error{Provider: "claude", Kind: llm.KindAccount, Message: "credit balance is too low"}}}
	_, err := NewLLMEnricher(&fakeCompleter{err: chainErr}).Enrich(context.Background(), richmondWarehousePermit)
	if !llm.IsExhausted(err) || llm.KindOf(err) != llm.KindAccount {
		t.Fatalf("expected the chain error to be wrapped, got %v", err)
	}
}

func TestLLMEnricher_DraftOutreach(t *testing.T) {
	fc := &fakeCompleter{resp: llm.Response{Text: "  Hello from Sandman YVR.\n"}}
	got, err := NewLLMEnricher(fc).DraftOutreach(context.Background(), storage.Lead{Title: "Bridge"})
	if err != nil || got != "Hello from Sandman YVR." {
		t.Fatalf("got %q, %v", got, err)
	}
	if fc.reqs[0].UseCase != "outreach" || fc.reqs[0].System != outreachSystemPrompt || !strings.Contains(fc.reqs[0].User, "Bridge") {
		t.Errorf("request = %+v", fc.reqs[0])
	}

	_, err = NewLLMEnricher(&fakeCompleter{err: errors.New("boom")}).DraftOutreach(context.Background(), storage.Lead{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnrichmentPrompt_routesBySource(t *testing.T) {
	for source, want := range map[string]func(collector.RawProject) string{
		"creativebc":       creativeBCPrompt,
		"vcc_events":       vccEventsPrompt,
		"google_news":      newsPrompt,
		"announcements":    announcementsPrompt,
		"eventbrite":       eventbritePrompt,
		"richmond_permits": permitPrompt,
		"bcbid":            permitPrompt,
	} {
		p := collector.RawProject{Source: source, Title: "t"}
		if enrichmentPrompt(p) != want(p) {
			t.Errorf("%s: wrong prompt", source)
		}
	}
}
