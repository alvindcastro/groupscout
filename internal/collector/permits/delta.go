package permits

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/logger"
)

// deltaHeaderRe identifies the start of a new Delta permit record.
// Delta PDFs (with -layout) produce headers like:
//
//	Mar 3, 2026  BP022784 MCRAE, ALICIA                      0.00    515,000.00
//
// The pattern anchors on the month abbreviation + day + year + permit number.
var deltaHeaderRe = regexp.MustCompile(
	`^(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2},\s+\d{4}\s+([A-Z]{2,3}\d{5,7})`)

// deltaDateRe extracts the date portion from a Delta header line.
var deltaDateRe = regexp.MustCompile(
	`^((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2},\s+\d{4})`)

// deltaPermitNumRe extracts the permit number from a Delta header line.
var deltaPermitNumRe = regexp.MustCompile(`[A-Z]{2,3}\d{5,7}`)

// deltaValueRe matches decimal numbers like 515,000.00 or 0.00 on a header line.
// The building value is always the last such number on the line.
var deltaValueRe = regexp.MustCompile(`[\d,]+\.\d{2}`)

// deltaTypeRe matches "Type INDUSTRIAL - TILBURY" lines.
var deltaTypeRe = regexp.MustCompile(`(?i)^\s*Type\s+(.+)`)

// deltaPurposeRe matches "Purpose Interior Tenant Improvement" lines.
var deltaPurposeRe = regexp.MustCompile(`(?i)^\s*Purpose\s+(.+)`)

// deltaCivicRe matches "Civic Address: 6705 DENNETT PL" lines.
// The address sometimes appears twice (with different folio numbers); we take the last non-empty one.
var deltaCivicRe = regexp.MustCompile(`(?i)Civic\s+Address:\s*(.+)`)

// deltaRelevantTypes are the permit type prefixes (before " - ") that indicate
// commercial/industrial projects likely to generate crew lodging demand.
var deltaRelevantTypes = map[string]bool{
	"industrial":    true,
	"commercial":    true,
	"assembly":      true,
	"institutional": true,
}

// deltaRecord holds the raw fields from one Delta permit entry.
type deltaRecord struct {
	PermitNumber string
	Builder      string // applicant/owner name from the header line
	IssueDate    time.Time
	ValueCAD     int64
	TypeRaw      string // full type string, e.g. "INDUSTRIAL - TILBURY"
	TypePrefix   string // normalized first word group, e.g. "industrial"
	Purpose      string // work description
	CivicAddress string
}

// DeltaCollector fetches building permit data from the City of Delta, BC.
// Delta publishes a single PDF at a known URL (no listing page).
// The URL is configured via DELTA_PERMITS_URL and updated manually when a new report is published.
type DeltaCollector struct {
	URL      string
	client   *http.Client
	Verbose  bool
	MinValue int64
}

// NewDeltaCollector returns a DeltaCollector with a 30-second HTTP timeout.
func NewDeltaCollector(url string) *DeltaCollector {
	return &DeltaCollector{
		URL:      url,
		client:   &http.Client{Timeout: 30 * time.Second},
		MinValue: minPermitValueCAD,
	}
}

// Name satisfies the collector.Collector interface.
func (d *DeltaCollector) Name() string { return "delta_permits" }

// Collect satisfies the collector.Collector interface. Downloads the Delta permit PDF,
// parses all permit records, and returns them as RawProjects.
func (d *DeltaCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	if d.URL == "" {
		return nil, fmt.Errorf("delta: DELTA_PERMITS_URL is not set")
	}

	path, rawData, cleanup, err := d.downloadPDF(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	records, err := parseDeltaPDF(path)
	if err != nil {
		return nil, err
	}

	if d.Verbose {
		logger.Log.Info("parsed records from PDF", "source", "delta", "count", len(records))
		counts := make(map[string]int)
		for _, rec := range records {
			counts[rec.TypePrefix]++
		}
		for t, n := range counts {
			logger.Log.Debug("permits by type", "source", "delta", "type", t, "count", n)
		}
	}

	var projects []collector.RawProject
	var skippedValue, skippedType int
	for _, rec := range records {
		if rec.ValueCAD <= d.MinValue {
			skippedValue++
			continue
		}
		if !deltaRelevantTypes[strings.ToLower(rec.TypePrefix)] {
			skippedType++
			continue
		}
		p := toDeltaRawProject(rec, rawData)
		p.SourceURL = d.URL
		p.Hash = hashDeltaPermit(rec.PermitNumber, rec.CivicAddress, rec.IssueDate)
		projects = append(projects, p)
	}

	if d.Verbose {
		logger.Log.Info("filtering complete",
			"source", "delta",
			"passed", len(projects),
			"skipped_low_value", skippedValue,
			"skipped_residential", skippedType,
			"min_value", d.MinValue,
		)
	}

	return projects, nil
}

// downloadPDF fetches the Delta permit PDF and returns a temp file path, raw bytes, plus cleanup func.
func (d *DeltaCollector) downloadPDF(ctx context.Context) (path string, data []byte, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("delta: build request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("delta: download pdf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, nil, fmt.Errorf("delta: pdf download returned HTTP %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, fmt.Errorf("delta: read pdf body: %w", err)
	}

	tmp, err := os.CreateTemp("", "delta-*.pdf")
	if err != nil {
		return "", nil, nil, fmt.Errorf("delta: create temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, nil, fmt.Errorf("delta: write pdf: %w", err)
	}
	tmp.Close()

	return tmp.Name(), data, func() { os.Remove(tmp.Name()) }, nil
}

// parseDeltaPDF extracts permit records from a Delta permit PDF.
// Uses pdftotext with -layout to preserve column alignment, which is critical
// for extracting the inline building value from the header line.
func parseDeltaPDF(path string) ([]deltaRecord, error) {
	pdftotext, err := findPdftotext()
	if err != nil {
		return nil, fmt.Errorf("delta: %w", err)
	}

	// -layout preserves the columnar layout so the building value stays on the header line.
	out, err := exec.Command(pdftotext, "-layout", path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("delta: pdftotext: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		lines = append(lines, scanner.Text()) // preserve raw lines — indentation matters
	}

	return parseDeltaPermitLines(lines), nil
}

// parseDeltaPermitLines converts pdftotext -layout output into deltaRecord values.
//
// Delta PDF format (with -layout):
//
//	Mar 3, 2026  BP022784 MCRAE, ALICIA                           0.00    515,000.00
//	             Type INDUSTRIAL - TILBURY
//	             Purpose Interior Tenant Improvement
//	             Folio: 344-932-31-0       Civic Address: 6705 DENNETT PL
//
// The header line contains: date, permit number, builder name, area (m²), building value (CAD).
// Continuation lines are indented (leading whitespace). We take the LAST decimal number on the
// header line as the building value (the previous number is building area in m²).
func parseDeltaPermitLines(lines []string) []deltaRecord {
	var records []deltaRecord
	var current *deltaRecord

	flush := func() {
		if current != nil {
			records = append(records, *current)
			current = nil
		}
	}

	for _, raw := range lines {
		// Header line: starts with a month name (no leading whitespace)
		if deltaHeaderRe.MatchString(raw) {
			flush()

			dateStr := ""
			if m := deltaDateRe.FindString(raw); m != "" {
				dateStr = m
			}
			permitNum := ""
			if m := deltaPermitNumRe.FindString(raw); m != "" {
				permitNum = m
			}

			// Extract builder name: text between permit number and the numeric columns.
			// Remove date, permit number, and trailing decimal numbers to isolate the name.
			builder := extractDeltaBuilder(raw, permitNum)

			// Building value = last decimal number on the line (area comes first).
			valueCAD := int64(0)
			allNums := deltaValueRe.FindAllString(raw, -1)
			if len(allNums) > 0 {
				valueCAD = parseDeltaDecimal(allNums[len(allNums)-1])
			}

			var issueDate time.Time
			if dateStr != "" {
				if t, err := time.Parse("Jan 2, 2006", dateStr); err == nil {
					issueDate = t
				}
			}

			current = &deltaRecord{
				PermitNumber: permitNum,
				Builder:      builder,
				IssueDate:    issueDate,
				ValueCAD:     valueCAD,
			}
			continue
		}

		if current == nil {
			continue
		}

		// Continuation lines are indented — trim for matching but require leading space.
		if len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t' {
			// Non-indented non-header line — not a continuation; skip.
			continue
		}

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		if m := deltaTypeRe.FindStringSubmatch(trimmed); m != nil {
			current.TypeRaw = strings.TrimSpace(m[1])
			// Normalize: take the part before " - " for category matching
			parts := strings.SplitN(current.TypeRaw, " - ", 2)
			current.TypePrefix = strings.ToLower(strings.TrimSpace(parts[0]))
			continue
		}

		if m := deltaPurposeRe.FindStringSubmatch(trimmed); m != nil {
			current.Purpose = strings.TrimSpace(m[1])
			continue
		}

		if m := deltaCivicRe.FindStringSubmatch(trimmed); m != nil {
			addr := strings.TrimSpace(m[1])
			if addr != "" {
				current.CivicAddress = addr // last non-empty address wins (Delta repeats it)
			}
			continue
		}
	}

	flush()
	return records
}

// extractDeltaBuilder isolates the builder/owner name from a Delta header line.
// The header is: "{date}  {permitNum} {BUILDER NAME}    {area}  {value}"
// We strip the date, permit number, and trailing decimal numbers.
func extractDeltaBuilder(line, permitNum string) string {
	// Remove everything up to and including the permit number
	idx := strings.Index(line, permitNum)
	if idx == -1 {
		return ""
	}
	rest := line[idx+len(permitNum):]

	// Strip trailing decimal numbers (area and value columns)
	rest = deltaValueRe.ReplaceAllString(rest, "")

	return strings.TrimSpace(rest)
}

// parseDeltaDecimal parses "515,000.00" → 515000 (truncates cents).
func parseDeltaDecimal(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	if dot := strings.Index(s, "."); dot != -1 {
		s = s[:dot]
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// isDeltaRelevant returns true if a Delta permit is worth enriching.
func isDeltaRelevant(rec deltaRecord, minValue int64) bool {
	if rec.ValueCAD <= minValue {
		return false
	}
	return deltaRelevantTypes[rec.TypePrefix]
}

// toDeltaRawProject maps a deltaRecord to the normalized collector.RawProject used by the pipeline.
func toDeltaRawProject(rec deltaRecord, rawData []byte) collector.RawProject {
	title := rec.TypeRaw
	if rec.CivicAddress != "" {
		title = fmt.Sprintf("%s — %s", rec.TypeRaw, rec.CivicAddress)
	}
	location := rec.CivicAddress
	if location == "" {
		location = "Delta, BC"
	}

	return collector.RawProject{
		Source:      "delta_permits",
		ExternalID:  rec.PermitNumber,
		Title:       title,
		Location:    location + ", Delta BC",
		Value:       rec.ValueCAD,
		Description: fmt.Sprintf("Work: %s | Builder: %s", rec.Purpose, rec.Builder),
		IssuedAt:    rec.IssueDate,
		RawData:     rawData,
		RawType:     "application/pdf",
	}
}

// hashDeltaPermit produces a deterministic dedup key for a Delta permit.
func hashDeltaPermit(permitNumber, address string, issuedAt time.Time) string {
	h := sha256.Sum256([]byte(
		"delta_permits|" + permitNumber + "|" + address + "|" + issuedAt.Format("2006-01-02"),
	))
	return fmt.Sprintf("%x", h)
}
