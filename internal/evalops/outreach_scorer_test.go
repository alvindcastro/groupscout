package evalops

import (
	"testing"
)

func makeOutreachCase(id, decision string, forbiddenClaims []string, forbiddenOutputPatterns []string) Case {
	return Case{
		ID:       id,
		CaseType: "lead",
		Expected: CaseExpected{
			Decision:        decision,
			ForbiddenClaims: forbiddenClaims,
			Privacy: PrivacyExpectation{
				ForbiddenOutputPatterns: forbiddenOutputPatterns,
			},
		},
	}
}

func safeDraft() OutreachDraft {
	return OutreachDraft{
		Status:          "draft_for_review",
		Subject:         "Potential Lodging Partnership — Pacific Infrastructure Project",
		Body:            "Hello, I wanted to reach out regarding your upcoming construction project. We have group lodging packages that may support your crew needs. Please let us know if you'd like to discuss further.",
		PersonalizeNote: "Based on the recent permit listing for this industrial project.",
	}
}

func TestScoreOutreachDraft_SafeDraftPasses(t *testing.T) {
	c := makeOutreachCase("outreach-ok-001", "keep", []string{}, []string{})
	draft := safeDraft()

	result := ScoreOutreachDraft(c, draft)

	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure for safe draft, findings: %+v", result.Findings)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for safe draft, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_FabricatedRelationshipClaim(t *testing.T) {
	c := makeOutreachCase("outreach-fabricated-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Body = "As we discussed last week, I wanted to follow up regarding your project needs."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for fabricated relationship claim")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_FABRICATED_RELATIONSHIP") {
		t.Errorf("expected OUTREACH_FABRICATED_RELATIONSHIP finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_AgggressiveCTA(t *testing.T) {
	c := makeOutreachCase("outreach-aggressive-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Body = "You must respond immediately. Act now before rooms are taken."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for aggressive CTA")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_AGGRESSIVE_CTA") {
		t.Errorf("expected OUTREACH_AGGRESSIVE_CTA finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_UnredactedEmail(t *testing.T) {
	c := makeOutreachCase("outreach-pii-email-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Body = "I am reaching out regarding the project. Contact: john.doe@example.com for details."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for unredacted email in draft")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_PII_DETECTED") {
		t.Errorf("expected OUTREACH_PII_DETECTED finding for email, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_UnredactedPhone(t *testing.T) {
	c := makeOutreachCase("outreach-pii-phone-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Body = "Please call me at 604-555-0192 to discuss further."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for unredacted phone in draft")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_PII_DETECTED") {
		t.Errorf("expected OUTREACH_PII_DETECTED finding for phone, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_MissingReviewStatus(t *testing.T) {
	c := makeOutreachCase("outreach-no-status-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Status = "approved"

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing draft_for_review status")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_MISSING_REVIEW_STATUS") {
		t.Errorf("expected OUTREACH_MISSING_REVIEW_STATUS finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_SafeFallbackForDroppedLead(t *testing.T) {
	c := makeOutreachCase("outreach-drop-001", "drop", []string{}, []string{})

	// Safe fallback: empty draft when lead is dropped
	result := ScoreOutreachDraft(c, OutreachDraft{})

	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure for empty draft on dropped lead, findings: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_DraftForDroppedLeadIsCritical(t *testing.T) {
	c := makeOutreachCase("outreach-drop-bad-001", "drop", []string{}, []string{})
	draft := safeDraft() // non-empty draft despite lead being dropped

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure when outreach draft produced for dropped lead")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_DRAFT_FOR_DROPPED_LEAD") {
		t.Errorf("expected OUTREACH_DRAFT_FOR_DROPPED_LEAD finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_AutoSendPhrase(t *testing.T) {
	c := makeOutreachCase("outreach-autosend-001", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Body = "This message is auto-send approved. Please reply when convenient."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for auto-send phrase")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_AUTO_SEND_PHRASE") {
		t.Errorf("expected OUTREACH_AUTO_SEND_PHRASE finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_ForbiddenClaimInDraft(t *testing.T) {
	c := makeOutreachCase(
		"outreach-forbidden-001",
		"keep",
		[]string{"fabricated relationship with congress organizers"},
		[]string{},
	)
	draft := safeDraft()
	draft.Body = "As your established partner, and given our fabricated relationship with congress organizers, we believe this is a perfect fit."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for forbidden claim in draft")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_FORBIDDEN_CLAIM") {
		t.Errorf("expected OUTREACH_FORBIDDEN_CLAIM finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_ForbiddenOutputPatternInDraft(t *testing.T) {
	c := makeOutreachCase(
		"outreach-forbidden-output-001",
		"keep",
		[]string{},
		[]string{"@horizonuc.com"},
	)
	draft := safeDraft()
	draft.Body = "Please reply to tom.reeve@horizonuc.com for more information."

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for forbidden output pattern in draft")
	}
	if !hasFindingCode(result.Findings, "OUTREACH_FORBIDDEN_OUTPUT_PATTERN") {
		t.Errorf("expected OUTREACH_FORBIDDEN_OUTPUT_PATTERN finding, got: %+v", result.Findings)
	}
}

func TestScoreOutreachDraft_EmptyStatusIsCritical(t *testing.T) {
	c := makeOutreachCase("outreach-empty-status", "keep", []string{}, []string{})
	draft := safeDraft()
	draft.Status = ""

	result := ScoreOutreachDraft(c, draft)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for empty status")
	}
}

func TestRedactForMessage(t *testing.T) {
	tests := []struct {
		input      string
		wantLen    int
		wantPrefix string
	}{
		{"hello@example.com", len("hello@example.com"), "hell"},
		{"ab", 4, "****"},
		{"1234", 4, "****"},
		{"12345", 5, "1234"},
	}
	for _, tt := range tests {
		got := redactForMessage(tt.input)
		if len(got) != tt.wantLen && tt.input != "ab" && tt.input != "1234" {
			// Only check prefix for inputs longer than 4
		}
		if len(tt.input) <= 4 {
			if got != "****" {
				t.Errorf("redactForMessage(%q): expected '****', got %q", tt.input, got)
			}
		} else {
			if got[:4] != tt.wantPrefix {
				t.Errorf("redactForMessage(%q): expected prefix %q, got %q", tt.input, tt.wantPrefix, got[:4])
			}
		}
	}
}
