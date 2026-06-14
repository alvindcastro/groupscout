package evalops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ReviewSample is a redacted production failure sample written by the review
// sample writer. It is safe for logs and can be converted into a draft case
// via DraftCasesFromReviewSamples.
type ReviewSample struct {
	TraceID         string            `json:"trace_id"`
	Stage           Stage             `json:"stage"`
	SourceSystem    string            `json:"source_system"`
	SourceType      string            `json:"source_type,omitempty"`
	FailureReason   string            `json:"failure_reason"`
	IssueClass      string            `json:"issue_class"`
	RawExcerpt      string            `json:"raw_excerpt,omitempty"`
	ReviewRequired  bool              `json:"review_required"`
	CapturedAt      string            `json:"captured_at"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// ReviewSampleWriter writes redacted review samples to a JSONL file.
// Samples are appended one per line. The writer enforces field redaction
// and rejects samples that contain unredacted sensitive data.
type ReviewSampleWriter struct {
	path    string
	enabled bool
	maxSize int64 // maximum file size in bytes before rotation is needed
}

// NewReviewSampleWriter creates a writer that appends to the given path.
// When enabled=false the writer is a no-op (disabled by config).
// maxSizeBytes=0 disables size checking.
func NewReviewSampleWriter(path string, enabled bool, maxSizeBytes int64) *ReviewSampleWriter {
	return &ReviewSampleWriter{
		path:    path,
		enabled: enabled,
		maxSize: maxSizeBytes,
	}
}

// Write appends a redacted ReviewSample to the JSONL file.
// Returns an error if the sample contains unredacted sensitive fields,
// the file cannot be opened, or the file exceeds the configured size limit.
func (w *ReviewSampleWriter) Write(s ReviewSample) error {
	if !w.enabled {
		return nil
	}

	// Validate and redact sensitive fields before writing
	if err := validateSample(s); err != nil {
		return fmt.Errorf("review sample writer: %w", err)
	}

	// Apply redaction to text fields
	s = redactSample(s)

	// Re-check after redaction
	if err := validateSampleNoSecrets(s); err != nil {
		return fmt.Errorf("review sample writer: redaction incomplete: %w", err)
	}

	if w.maxSize > 0 {
		if info, err := os.Stat(w.path); err == nil {
			if info.Size() > w.maxSize {
				return fmt.Errorf("review sample writer: file %q exceeds max size %d bytes (rotate before writing)", w.path, w.maxSize)
			}
		}
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("review sample writer: open %q: %w", w.path, err)
	}
	defer f.Close()

	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("review sample writer: marshal: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%s\n", b); err != nil {
		return fmt.Errorf("review sample writer: write: %w", err)
	}
	return nil
}

func validateSample(s ReviewSample) error {
	if s.TraceID == "" {
		return fmt.Errorf("trace_id is required")
	}
	if s.FailureReason == "" {
		return fmt.Errorf("failure_reason is required")
	}
	if !s.ReviewRequired {
		return fmt.Errorf("review_required must be true for all samples")
	}
	return nil
}

func redactSample(s ReviewSample) ReviewSample {
	s.FailureReason = RedactSensitive(s.FailureReason)
	s.RawExcerpt = TruncateRawText(s.RawExcerpt)
	redactedMeta := make(map[string]string, len(s.Metadata))
	for k, v := range s.Metadata {
		redactedMeta[k] = RedactSensitive(v)
	}
	s.Metadata = redactedMeta
	return s
}

func validateSampleNoSecrets(s ReviewSample) error {
	fields := []string{s.FailureReason, s.RawExcerpt}
	for _, v := range s.Metadata {
		fields = append(fields, v)
	}
	for _, f := range fields {
		for _, pat := range redactPatterns {
			if pat.MatchString(f) {
				return fmt.Errorf("sensitive pattern found in field value")
			}
		}
	}
	return nil
}

// ReadSamples reads all ReviewSample records from the given JSONL file path.
func ReadSamples(path string) ([]ReviewSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read samples: open %q: %w", path, err)
	}
	defer f.Close()

	var samples []ReviewSample
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var s ReviewSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, fmt.Errorf("read samples: %s:%d: malformed JSON: %w", path, lineNum, err)
		}
		samples = append(samples, s)
	}
	return samples, scanner.Err()
}

// DraftCase is a JSONL case draft generated from a ReviewSample.
// Expected fields contain TODO_REVIEW_* placeholders requiring human review.
type DraftCase struct {
	ID             string            `json:"id"`
	CaseType       string            `json:"case_type"`
	Category       string            `json:"category"`
	RiskLevel      string            `json:"risk_level"`
	TraceID        string            `json:"trace_id"`
	ReviewRequired bool              `json:"review_required"`
	Source         CaseSource        `json:"source"`
	Raw            map[string]any    `json:"raw"`
	Expected       DraftExpected     `json:"expected"`
	Notes          string            `json:"notes"`
}

// DraftExpected holds the expected fields for a draft case with TODO placeholders.
type DraftExpected struct {
	EvalResult         string `json:"eval_result"`
	Decision           string `json:"decision"`
	ReleaseBlocking    bool   `json:"release_blocking"`
	SeverityOnMismatch string `json:"severity_on_mismatch"`
}

// DraftCasesFromReviewSamples converts redacted ReviewSamples into draft
// JSONL cases with TODO_REVIEW_* placeholders. Duplicate trace IDs get
// numeric suffixes. All generated drafts have review_required=true and
// release_blocking=false. Drafts must not be placed in data/evals/groupscout/
// until a human has replaced all TODO fields.
func DraftCasesFromReviewSamples(samples []ReviewSample) ([]DraftCase, error) {
	if len(samples) == 0 {
		return nil, nil
	}

	seen := make(map[string]int)
	var drafts []DraftCase

	for _, s := range samples {
		if s.TraceID == "" {
			return nil, fmt.Errorf("sample missing trace_id")
		}

		// Generate a draft ID from the trace ID
		baseID := "draft-" + sanitizeID(s.TraceID)
		id := baseID
		if n := seen[baseID]; n > 0 {
			id = fmt.Sprintf("%s-%d", baseID, n+1)
		}
		seen[baseID]++

		caseType := "lead"
		if s.Stage == StageAlert {
			caseType = "alert"
		}

		now := time.Now().UTC().Format(time.RFC3339)

		draft := DraftCase{
			ID:             id,
			CaseType:       caseType,
			Category:       s.IssueClass,
			RiskLevel:      "warning",
			TraceID:        s.TraceID,
			ReviewRequired: true,
			Source: CaseSource{
				System:      s.SourceSystem,
				Type:        s.SourceType,
				FixtureURL:  "https://fixtures.groupscout.local/draft/" + sanitizeID(s.TraceID),
				CollectedAt: s.CapturedAt,
			},
			Raw: map[string]any{
				"title":    "TODO_REVIEW_title",
				"text":     SafeRawExcerpt(s.RawExcerpt),
				"metadata": map[string]any{"trace_id": s.TraceID},
			},
			Expected: DraftExpected{
				EvalResult:         "TODO_REVIEW_eval_result",
				Decision:           "TODO_REVIEW_decision",
				ReleaseBlocking:    false,
				SeverityOnMismatch: "warning",
			},
			Notes: fmt.Sprintf(
				"Draft generated from production sample. trace_id=%s stage=%s issue_class=%s captured_at=%s. REPLACE all TODO_REVIEW_* fields before promoting to golden set.",
				s.TraceID, s.Stage, s.IssueClass, now,
			),
		}
		drafts = append(drafts, draft)
	}

	return drafts, nil
}

// WriteDraftCasesJSONL writes draft cases to the given path, one per line.
// The caller is responsible for keeping this file outside data/evals/groupscout/
// until all TODO fields have been reviewed.
func WriteDraftCasesJSONL(path string, drafts []DraftCase) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write draft cases: create %q: %w", path, err)
	}
	defer f.Close()

	for _, d := range drafts {
		b, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("write draft cases: marshal %q: %w", d.ID, err)
		}
		if _, err := fmt.Fprintf(f, "%s\n", b); err != nil {
			return fmt.Errorf("write draft cases: write %q: %w", d.ID, err)
		}
	}
	return nil
}

// sanitizeID converts a trace ID into a safe case ID segment.
func sanitizeID(s string) string {
	var sb strings.Builder
	for _, ch := range strings.ToLower(s) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sb.WriteRune(ch)
		} else {
			sb.WriteRune('-')
		}
	}
	result := sb.String()
	// Trim leading/trailing dashes
	result = strings.Trim(result, "-")
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}
