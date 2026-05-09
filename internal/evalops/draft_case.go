package evalops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TODOReviewEvalResult              = "TODO_REVIEW_EVAL_RESULT"
	TODOReviewDecision                = "TODO_REVIEW_DECISION"
	TODOReviewEvidenceClaim           = "TODO_REVIEW_EVIDENCE_CLAIM"
	TODOReviewEvidenceSupport         = "TODO_REVIEW_EVIDENCE_SUPPORT"
	TODOReviewProjectType             = "TODO_REVIEW_PROJECT_TYPE"
	TODOReviewLodgingNeed             = "TODO_REVIEW_LODGING_NEED"
	TODOReviewConfidence              = "TODO_REVIEW_CONFIDENCE"
	TODOReviewAlertState              = "TODO_REVIEW_ALERT_STATE"
	TODOReviewForbiddenClaim          = "TODO_REVIEW_FORBIDDEN_CLAIM"
	TODOReviewPrivacyMustRedact       = "TODO_REVIEW_PRIVACY_MUST_REDACT"
	TODOReviewPrivacyForbiddenPattern = "TODO_REVIEW_PRIVACY_FORBIDDEN_PATTERN"
	TODOReviewCriticalFailure         = "TODO_REVIEW_CRITICAL_FAILURE"
	TODOReviewCategory                = "TODO_REVIEW_CATEGORY"
	TODOReviewRawText                 = "TODO_REVIEW_RAW_TEXT"
	TODOReviewSourceSystem            = "TODO_REVIEW_SOURCE_SYSTEM"
	TODOReviewSourceType              = "TODO_REVIEW_SOURCE_TYPE"
	TODOReviewTimestamp               = "TODO_REVIEW_TIMESTAMP"
)

var ErrDraftCaseReviewRequired = errors.New("draft case review required")

type DraftCase struct {
	Case
	TraceID        string `json:"trace_id"`
	ReviewRequired bool   `json:"review_required"`
}

type DraftCaseOptions struct {
	ExistingCaseIDs map[string]struct{}
	CaseIDPrefix    string
	MaxFieldBytes   int
}

func DraftCasesFromReviewSamples(samples []ReviewSample, options DraftCaseOptions) ([]DraftCase, error) {
	if options.CaseIDPrefix == "" {
		options.CaseIDPrefix = "draft"
	}
	if options.MaxFieldBytes <= 0 {
		options.MaxFieldBytes = 1024
	}

	used := make(map[string]struct{}, len(options.ExistingCaseIDs)+len(samples))
	for id := range options.ExistingCaseIDs {
		used[id] = struct{}{}
	}

	drafts := make([]DraftCase, 0, len(samples))
	for i, sample := range samples {
		if !sample.ReviewRequired {
			return nil, fmt.Errorf("%w: sample %d trace_id %q", ErrDraftCaseReviewRequired, i+1, sample.TraceID)
		}
		if err := rejectUnsafeSample(sample); err != nil {
			return nil, err
		}

		safe := sample.safe(options.MaxFieldBytes)
		id := uniqueDraftCaseID(safe, options.CaseIDPrefix, i+1, used)
		used[id] = struct{}{}

		draft := DraftCase{
			Case: Case{
				ID:        id,
				CaseType:  draftCaseType(safe),
				Category:  draftString(safe.Attributes["category"], TODOReviewCategory),
				RiskLevel: draftSeverity(safe.Severity),
				Source:    draftSource(safe, id),
				Raw:       draftRawPayload(safe),
				Notes:     draftNotes(safe),
			},
			TraceID:        safe.TraceID,
			ReviewRequired: true,
		}
		draft.Expected = draftExpectedOutcome(draft.CaseType, draft.RiskLevel)
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func WriteDraftCasesJSONL(path string, drafts []DraftCase) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("draft case path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, draft := range drafts {
		draft.ReviewRequired = true
		draft.Expected.ReleaseBlocking = false
		if err := encoder.Encode(draft); err != nil {
			return err
		}
	}
	return nil
}

func uniqueDraftCaseID(sample ReviewSample, prefix string, ordinal int, used map[string]struct{}) string {
	seed := sample.CaseID
	if seed == "" {
		seed = sample.TraceID
	}
	if seed == "" {
		seed = fmt.Sprintf("sample-%d", ordinal)
	}
	base := strings.Trim(slugify(seed), "-")
	if base == "" {
		base = fmt.Sprintf("sample-%d", ordinal)
	}
	base = strings.TrimSuffix(strings.TrimSpace(prefix), "-") + "-" + base
	candidate := base
	for suffix := 2; ; suffix++ {
		if _, exists := used[candidate]; !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func draftCaseType(sample ReviewSample) CaseType {
	value := strings.ToLower(strings.TrimSpace(sample.Attributes["case_type"]))
	switch CaseType(value) {
	case CaseTypeLead:
		return CaseTypeLead
	case CaseTypeAlert:
		return CaseTypeAlert
	}
	if sample.Stage == TraceStageAlert || sample.SourceSystem == "yvr_disruption" || sample.SourceType == "multi_signal_snapshot" {
		return CaseTypeAlert
	}
	return CaseTypeLead
}

func draftSeverity(severity Severity) Severity {
	if validSeverity(severity) {
		return severity
	}
	return SeverityWarning
}

func draftSource(sample ReviewSample, draftID string) Source {
	collectedAt := sample.Attributes["collected_at"]
	if collectedAt == "" && !sample.CapturedAt.IsZero() {
		collectedAt = sample.CapturedAt.UTC().Format(time.RFC3339)
	}
	return Source{
		System:      draftString(firstNonEmpty(sample.SourceSystem, sample.Attributes["source_system"]), TODOReviewSourceSystem),
		Type:        draftString(firstNonEmpty(sample.SourceType, sample.Attributes["source_type"]), TODOReviewSourceType),
		FixtureURL:  draftString(sample.Attributes["fixture_url"], "https://fixtures.groupscout.local/drafts/"+draftID),
		CollectedAt: draftString(collectedAt, TODOReviewTimestamp),
	}
}

func draftRawPayload(sample ReviewSample) RawPayload {
	metadata := draftMetadata(sample)
	raw := RawPayload{
		Title:    sample.Attributes["title"],
		Text:     draftString(firstNonEmpty(sample.Attributes["source_excerpt"], sample.Attributes["raw_text"], sample.Attributes["text"]), TODOReviewRawText),
		IssuedAt: sample.Attributes["issued_at"],
		Location: sample.Attributes["location"],
		Metadata: metadata,
	}
	if value, ok := parseCAD(sample.Attributes["value_cad"]); ok {
		raw.ValueCAD = &value
	}
	if draftCaseType(sample) == CaseTypeAlert {
		raw.AirportCode = draftString(sample.Attributes["airport_code"], "YVR")
		raw.ObservedAt = draftString(firstNonEmpty(sample.Attributes["observed_at"], sample.Attributes["collected_at"]), TODOReviewTimestamp)
		raw.Signals = draftAlertSignals(sample.Attributes)
	}
	return raw
}

func draftMetadata(sample ReviewSample) map[string]any {
	metadata := map[string]any{
		"trace_id":        sample.TraceID,
		"review_required": true,
		"review_reason":   sample.Reason,
		"review_stage":    string(sample.Stage),
	}
	if sample.CaseID != "" {
		metadata["review_sample_case_id"] = sample.CaseID
	}
	if !sample.CapturedAt.IsZero() {
		metadata["captured_at"] = sample.CapturedAt.UTC().Format(time.RFC3339)
	}
	for key, value := range sample.Attributes {
		if key == "" || mappedDraftAttribute(key) {
			continue
		}
		metadata["sample_"+key] = value
	}
	return metadata
}

func draftAlertSignals(attrs map[string]string) AlertSignals {
	return AlertSignals{
		CancelledCount:     parseIntAttr(attrs["cancelled_count"]),
		TotalFlights:       parseIntAttr(attrs["total_flights"]),
		CancellationRate:   parseFloatAttr(attrs["cancellation_rate"]),
		AvgSeatsPerFlight:  parseIntAttr(attrs["avg_seats_per_flight"]),
		ConnectingPaxRatio: parseFloatAttr(attrs["connecting_pax_ratio"]),
		HourOfDay:          parseIntAttr(attrs["hour_of_day"]),
		MinutesActive:      parseIntAttr(attrs["minutes_active"]),
		SingleRunwayOps:    parseBoolAttr(attrs["single_runway_ops"]),
		FeedAgeMinutes:     parseIntAttr(attrs["feed_age_minutes"]),
	}
}

func draftExpectedOutcome(caseType CaseType, severity Severity) ExpectedOutcome {
	expected := ExpectedOutcome{
		EvalResult:         TODOReviewEvalResult,
		Decision:           TODOReviewDecision,
		ReleaseBlocking:    false,
		SeverityOnMismatch: severity,
		ScoreBand:          ScoreBand{Min: 0, Max: 0},
		Evidence: []EvidenceRequirement{{
			Claim:           TODOReviewEvidenceClaim,
			MustSupportWith: TODOReviewEvidenceSupport,
		}},
		ForbiddenClaims: []string{TODOReviewForbiddenClaim},
		Privacy: PrivacyExpectation{
			MustRedact:        []string{TODOReviewPrivacyMustRedact},
			ForbiddenPatterns: []string{TODOReviewPrivacyForbiddenPattern},
		},
		CriticalFailureIf: []string{TODOReviewCriticalFailure},
	}
	switch caseType {
	case CaseTypeAlert:
		expected.Alert = &AlertExpectation{
			State:           TODOReviewAlertState,
			Priority:        false,
			StaleData:       false,
			RequiredSignals: []string{TODOReviewEvidenceSupport},
		}
	default:
		expected.Enrichment = &EnrichmentExpectation{
			ProjectType: TODOReviewProjectType,
			EstimatedRoomNights: RangeExpectation{
				UnknownAllowed: true,
			},
			ProjectDurationMonths: RangeExpectation{
				UnknownAllowed: true,
			},
			LodgingNeed: TODOReviewLodgingNeed,
			Confidence:  TODOReviewConfidence,
		}
	}
	return expected
}

func draftNotes(sample ReviewSample) string {
	parts := []string{"Draft generated from redacted review sample; human review required before promotion."}
	if sample.TraceID != "" {
		parts = append(parts, "trace_id="+sample.TraceID)
	}
	if sample.CaseID != "" {
		parts = append(parts, "sample_case_id="+sample.CaseID)
	}
	if sample.Reason != "" {
		parts = append(parts, "reason="+sample.Reason)
	}
	return strings.Join(parts, " ")
}

func mappedDraftAttribute(key string) bool {
	switch key {
	case "case_type", "category", "title", "source_excerpt", "raw_text", "text", "issued_at", "location", "value_cad", "airport_code", "observed_at", "collected_at", "source_system", "source_type", "fixture_url":
		return true
	default:
		return false
	}
}

func parseCAD(value string) (int64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}

func parseIntAttr(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func parseFloatAttr(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func parseBoolAttr(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}

func draftString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = draftCaseSlugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

var draftCaseSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)
