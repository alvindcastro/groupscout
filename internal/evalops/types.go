package evalops

type CaseType string

const (
	CaseTypeLead  CaseType = "lead"
	CaseTypeAlert CaseType = "alert"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type ResultStatus string

const (
	ResultPass ResultStatus = "pass"
	ResultFail ResultStatus = "fail"
)

type Case struct {
	ID        string          `json:"id"`
	CaseType  CaseType        `json:"case_type"`
	Category  string          `json:"category"`
	RiskLevel Severity        `json:"risk_level"`
	Source    Source          `json:"source"`
	Raw       RawPayload      `json:"raw"`
	Expected  ExpectedOutcome `json:"expected"`
	Notes     string          `json:"notes,omitempty"`
}

type Source struct {
	System      string `json:"system"`
	Type        string `json:"type"`
	FixtureURL  string `json:"fixture_url"`
	CollectedAt string `json:"collected_at"`
}

type RawPayload struct {
	Title       string         `json:"title,omitempty"`
	Text        string         `json:"text"`
	IssuedAt    string         `json:"issued_at,omitempty"`
	Location    string         `json:"location,omitempty"`
	ValueCAD    *int64         `json:"value_cad,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	AirportCode string         `json:"airport_code,omitempty"`
	ObservedAt  string         `json:"observed_at,omitempty"`
	Signals     AlertSignals   `json:"signals,omitempty"`
}

type AlertSignals struct {
	CancelledCount     int            `json:"cancelled_count"`
	TotalFlights       int            `json:"total_flights"`
	CancellationRate   float64        `json:"cancellation_rate"`
	AvgSeatsPerFlight  int            `json:"avg_seats_per_flight"`
	ConnectingPaxRatio float64        `json:"connecting_pax_ratio"`
	HourOfDay          int            `json:"hour_of_day"`
	MinutesActive      int            `json:"minutes_active"`
	WeatherAlert       *WeatherSignal `json:"weather_alert"`
	Notams             *[]NotamSignal `json:"notams"`
	SingleRunwayOps    bool           `json:"single_runway_ops"`
	FeedAgeMinutes     int            `json:"feed_age_minutes"`
}

type WeatherSignal struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
}

type NotamSignal struct {
	Type           string `json:"type"`
	Impact         string `json:"impact"`
	CapacityImpact string `json:"capacity_impact"`
}

type ExpectedOutcome struct {
	EvalResult         string                 `json:"eval_result"`
	Decision           string                 `json:"decision"`
	ReleaseBlocking    bool                   `json:"release_blocking"`
	SeverityOnMismatch Severity               `json:"severity_on_mismatch"`
	ScoreBand          ScoreBand              `json:"score_band"`
	Enrichment         *EnrichmentExpectation `json:"enrichment"`
	Alert              *AlertExpectation      `json:"alert"`
	Evidence           []EvidenceRequirement  `json:"evidence"`
	ForbiddenClaims    []string               `json:"forbidden_claims"`
	Privacy            PrivacyExpectation     `json:"privacy"`
	CriticalFailureIf  []string               `json:"critical_failure_if"`
}

type ScoreBand struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type EnrichmentExpectation struct {
	ProjectType           string           `json:"project_type"`
	EstimatedRoomNights   RangeExpectation `json:"estimated_room_nights"`
	ProjectDurationMonths RangeExpectation `json:"project_duration_months"`
	LodgingNeed           string           `json:"lodging_need"`
	Confidence            string           `json:"confidence"`
}

type RangeExpectation struct {
	Min            int  `json:"min"`
	Max            int  `json:"max"`
	UnknownAllowed bool `json:"unknown_allowed"`
}

type AlertExpectation struct {
	State           string   `json:"state"`
	Priority        bool     `json:"priority"`
	StaleData       bool     `json:"stale_data"`
	RequiredSignals []string `json:"required_signals"`
}

type EvidenceRequirement struct {
	Claim           string `json:"claim"`
	MustSupportWith string `json:"must_support_with"`
}

type PrivacyExpectation struct {
	MustRedact        []string `json:"must_redact"`
	ForbiddenPatterns []string `json:"forbidden_patterns"`
}

type Result struct {
	CaseID           string       `json:"case_id"`
	Scorer           string       `json:"scorer"`
	Status           ResultStatus `json:"status"`
	Severity         Severity     `json:"severity"`
	ReleaseBlocking  bool         `json:"release_blocking"`
	Decision         string       `json:"decision,omitempty"`
	ExpectedDecision string       `json:"expected_decision,omitempty"`
	Score            int          `json:"score,omitempty"`
	Message          string       `json:"message,omitempty"`
}

func passResult(c Case, scorer, decision string, score int, message string) Result {
	return Result{
		CaseID:           c.ID,
		Scorer:           scorer,
		Status:           ResultPass,
		Severity:         SeverityInfo,
		ReleaseBlocking:  false,
		Decision:         decision,
		ExpectedDecision: c.Expected.Decision,
		Score:            score,
		Message:          message,
	}
}

func failResult(c Case, scorer string, severity Severity, releaseBlocking bool, decision string, score int, message string) Result {
	return Result{
		CaseID:           c.ID,
		Scorer:           scorer,
		Status:           ResultFail,
		Severity:         severity,
		ReleaseBlocking:  releaseBlocking,
		Decision:         decision,
		ExpectedDecision: c.Expected.Decision,
		Score:            score,
		Message:          message,
	}
}

func mismatchResult(c Case, scorer, decision string, score int, message string) Result {
	severity := c.Expected.SeverityOnMismatch
	if severity == "" {
		severity = c.RiskLevel
	}
	if severity == "" {
		severity = SeverityCritical
	}
	return failResult(c, scorer, severity, c.Expected.ReleaseBlocking, decision, score, message)
}
