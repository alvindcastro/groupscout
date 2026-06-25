// Package evalops implements the AI quality EvalOps harness for groupscout.
// It loads golden fixture cases, runs deterministic scorers, and generates
// reports for CI gates. No live providers, Slack, Resend, Sentry, or external
// APIs are called from this package.
package evalops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ValidSourceSystems lists all known source system names accepted by the loader.
var ValidSourceSystems = map[string]bool{
	"richmond_permits": true,
	"delta_permits":    true,
	"creativebc":       true,
	"vcc":              true,
	"eventbrite":       true,
	"bcbid":            true,
	"announcements":    true,
	"yvr_disruption":   true,
}

// ValidCaseTypes are the accepted case_type values.
var ValidCaseTypes = map[string]bool{
	"lead":  true,
	"alert": true,
}

// ValidRiskLevels are the accepted risk_level values.
var ValidRiskLevels = map[string]bool{
	"critical": true,
	"warning":  true,
	"info":     true,
}

// ValidDecisions are the accepted decision values.
var ValidDecisions = map[string]bool{
	// Lead decisions
	"keep":         true,
	"drop":         true,
	"needs_review": true,
	// Alert decisions
	"ignore":      true,
	"watch":       true,
	"soft_alert":  true,
	"hard_alert":  true,
	"fail_closed": true,
}

// Case is a single golden or draft eval fixture case.
type Case struct {
	ID             string          `json:"id"`
	CaseType       string          `json:"case_type"`
	Category       string          `json:"category"`
	RiskLevel      string          `json:"risk_level"`
	Source         CaseSource      `json:"source"`
	Raw            json.RawMessage `json:"raw"`
	Expected       CaseExpected    `json:"expected"`
	Notes          string          `json:"notes,omitempty"`
	TraceID        string          `json:"trace_id,omitempty"`
	ReviewRequired bool            `json:"review_required,omitempty"`
}

// CaseSource holds the synthetic source metadata for a fixture case.
type CaseSource struct {
	System      string `json:"system"`
	Type        string `json:"type"`
	FixtureURL  string `json:"fixture_url"`
	CollectedAt string `json:"collected_at"`
}

// CaseExpected holds the expected behavior for a fixture case.
type CaseExpected struct {
	EvalResult         string              `json:"eval_result"`
	Decision           string              `json:"decision"`
	ReleaseBlocking    bool                `json:"release_blocking"`
	SeverityOnMismatch string              `json:"severity_on_mismatch"`
	ScoreBand          ScoreBand           `json:"score_band"`
	Enrichment         *ExpectedEnrichment `json:"enrichment"`
	Alert              *ExpectedAlert      `json:"alert"`
	Evidence           []string            `json:"evidence"`
	ForbiddenClaims    []string            `json:"forbidden_claims"`
	Privacy            PrivacyExpectation  `json:"privacy"`
	CriticalFailureIf  []string            `json:"critical_failure_if"`
}

// ScoreBand specifies the expected min/max inclusive score or SPS range.
type ScoreBand struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ExpectedEnrichment specifies expected AI enrichment fields for lead cases.
type ExpectedEnrichment struct {
	ProjectType           string         `json:"project_type"`
	EstimatedRoomNights   RoomNightRange `json:"estimated_room_nights"`
	ProjectDurationMonths DurationRange  `json:"project_duration_months"`
	LodgingNeed           string         `json:"lodging_need"`
	Confidence            string         `json:"confidence"`
}

// RoomNightRange specifies min/max room night estimate and whether unknown is allowed.
type RoomNightRange struct {
	Min            float64 `json:"min"`
	Max            float64 `json:"max"`
	UnknownAllowed bool    `json:"unknown_allowed"`
}

// DurationRange specifies min/max project duration and whether unknown is allowed.
type DurationRange struct {
	Min            float64 `json:"min"`
	Max            float64 `json:"max"`
	UnknownAllowed bool    `json:"unknown_allowed"`
}

// ExpectedAlert specifies expected alert state and signals for alert cases.
type ExpectedAlert struct {
	State           string   `json:"state"`
	Priority        string   `json:"priority"`
	StaleData       bool     `json:"stale_data"`
	RequiredSignals []string `json:"required_signals"`
}

// PrivacyExpectation specifies redaction and forbidden output patterns.
type PrivacyExpectation struct {
	RedactPatterns          []string `json:"redact_patterns"`
	ForbiddenOutputPatterns []string `json:"forbidden_output_patterns"`
}

// RawLead is the raw payload for a lead-type case.
type RawLead struct {
	Title    string         `json:"title"`
	Text     string         `json:"text"`
	IssuedAt string         `json:"issued_at"`
	Location string         `json:"location"`
	ValueCAD *float64       `json:"value_cad"`
	Metadata map[string]any `json:"metadata"`
}

// RawAlert is the raw payload for an alert-type case.
type RawAlert struct {
	AirportCode string       `json:"airport_code"`
	ObservedAt  string       `json:"observed_at"`
	Text        string       `json:"text"`
	Signals     AlertSignals `json:"signals"`
}

// AlertSignals captures the multi-signal snapshot for an alert case.
type AlertSignals struct {
	CancelledCount     int            `json:"cancelled_count"`
	TotalFlights       int            `json:"total_flights"`
	CancellationRate   float64        `json:"cancellation_rate"`
	AvgSeatsPerFlight  int            `json:"avg_seats_per_flight"`
	ConnectingPaxRatio float64        `json:"connecting_pax_ratio"`
	HourOfDay          int            `json:"hour_of_day"`
	MinutesActive      int            `json:"minutes_active"`
	WeatherAlert       *WeatherSignal `json:"weather_alert"`
	NotamActive        bool           `json:"notam_active"`
	SingleRunwayOps    bool           `json:"single_runway_ops"`
	FeedAgeSeconds     int            `json:"feed_age_seconds"`
}

// WeatherSignal captures weather alert data for an alert signal snapshot.
type WeatherSignal struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
}

// LoadError is a structured error from case loading, including file and line context.
type LoadError struct {
	File    string
	Line    int
	Message string
}

func (e LoadError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
}

// LoadCases loads JSONL fixture files from the given paths and returns
// a deterministically ordered slice of Cases. All validation errors are
// aggregated and returned together. Duplicate IDs across all files fail.
// Draft cases with TODO decisions are rejected.
func LoadCases(paths []string) ([]Case, error) {
	var cases []Case
	var errs []string
	seen := make(map[string]bool)

	for _, path := range paths {
		fileCases, fileErrs := loadFile(path, seen)
		cases = append(cases, fileCases...)
		errs = append(errs, fileErrs...)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("case loader: %d error(s):\n%s", len(errs), strings.Join(errs, "\n"))
	}
	return cases, nil
}

// LoadCasesFromDir loads all *.jsonl files from the given directory in
// lexicographic order. Returns the same error semantics as LoadCases.
func LoadCasesFromDir(dir string) ([]Case, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("case loader: glob %q: %w", dir, err)
	}
	sort.Strings(entries)
	return LoadCases(entries)
}

func loadFile(path string, seen map[string]bool) ([]Case, []string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s:0: cannot open file: %v", path, err)}
	}
	defer f.Close()

	var cases []Case
	var errs []string
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			errs = append(errs, (&LoadError{File: path, Line: lineNum, Message: fmt.Sprintf("malformed JSON: %v", err)}).Error())
			continue
		}

		if lineErrs := validateCase(c, path, lineNum, seen); len(lineErrs) > 0 {
			errs = append(errs, lineErrs...)
			continue
		}

		seen[c.ID] = true
		cases = append(cases, c)
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Sprintf("%s:0: scanner error: %v", path, err))
	}
	return cases, errs
}

func validateCase(c Case, path string, line int, seen map[string]bool) []string {
	var errs []string
	loc := func(msg string) string {
		return (&LoadError{File: path, Line: line, Message: msg}).Error()
	}

	if c.ID == "" {
		errs = append(errs, loc("missing required field: id"))
	} else if seen[c.ID] {
		errs = append(errs, loc(fmt.Sprintf("duplicate id: %q", c.ID)))
	}

	if !ValidCaseTypes[c.CaseType] {
		errs = append(errs, loc(fmt.Sprintf("unknown case_type: %q (must be one of: lead, alert)", c.CaseType)))
	}

	if !ValidRiskLevels[c.RiskLevel] {
		errs = append(errs, loc(fmt.Sprintf("unknown risk_level: %q (must be one of: critical, warning, info)", c.RiskLevel)))
	}

	if !ValidSourceSystems[c.Source.System] {
		errs = append(errs, loc(fmt.Sprintf("unknown source system: %q", c.Source.System)))
	}

	if c.Source.CollectedAt != "" {
		if _, err := time.Parse(time.RFC3339, c.Source.CollectedAt); err != nil {
			errs = append(errs, loc(fmt.Sprintf("source.collected_at must be RFC3339: %v", err)))
		}
	}

	if len(c.Raw) == 0 {
		errs = append(errs, loc("missing required field: raw"))
	}

	if c.Expected.Decision == "" {
		errs = append(errs, loc("missing required field: expected.decision"))
	} else if strings.HasPrefix(c.Expected.Decision, "TODO_REVIEW_") {
		errs = append(errs, loc(fmt.Sprintf("draft case with unreviewed decision %q — review required before golden set", c.Expected.Decision)))
	} else if !ValidDecisions[c.Expected.Decision] {
		errs = append(errs, loc(fmt.Sprintf("unknown decision: %q", c.Expected.Decision)))
	}

	if c.Expected.EvalResult == "" {
		errs = append(errs, loc("missing required field: expected.eval_result"))
	}

	return errs
}

// ParseRawLead decodes the raw field of a lead case into a RawLead.
func ParseRawLead(c Case) (*RawLead, error) {
	if c.CaseType != "lead" {
		return nil, fmt.Errorf("case %q is type %q, not lead", c.ID, c.CaseType)
	}
	var r RawLead
	if err := json.Unmarshal(c.Raw, &r); err != nil {
		return nil, fmt.Errorf("case %q: unmarshal raw lead: %w", c.ID, err)
	}
	return &r, nil
}

// ParseRawAlert decodes the raw field of an alert case into a RawAlert.
func ParseRawAlert(c Case) (*RawAlert, error) {
	if c.CaseType != "alert" {
		return nil, fmt.Errorf("case %q is type %q, not alert", c.ID, c.CaseType)
	}
	var r RawAlert
	if err := json.Unmarshal(c.Raw, &r); err != nil {
		return nil, fmt.Errorf("case %q: unmarshal raw alert: %w", c.ID, err)
	}
	return &r, nil
}
