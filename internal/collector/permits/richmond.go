package permits

import (
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

const (
	richmondBaseURL    = "https://www.richmond.ca"
	richmondReportsURL = "https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm"
)

// buildingReportRe matches building report PDF href paths on the weekly reports page.
// Example: /__shared/assets/buildingreportmarch15_202678649.pdf
var buildingReportRe = regexp.MustCompile(`/__shared/assets/buildingreport[^"]+\.pdf`)

// permitRecord holds the raw fields extracted from one permit entry in a Richmond PDF report.
// See parseBBoxLayout for how the cells are located.
type permitRecord struct {
	SubType      string // e.g. "Hotel", "Warehouse", "Office"
	FolderNumber string // e.g. "25 036523 000 00 B7"
	WorkProposed string // e.g. "New", "Alteration", "Revision"
	Status       string // e.g. "Issued"
	IssueDate    time.Time
	ValueCAD     int64  // construction value in CAD dollars
	Address      string // civic address + project description
	Applicant    string
	Contractor   string
}

// RichmondCollector scrapes building permit PDFs published weekly by the City of Richmond BC.
// Richmond has no open data API — data is only available as PDFs at:
// https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm
type RichmondCollector struct {
	client   *http.Client
	Verbose  bool  // when true, logs intermediate step counts to stderr
	MinValue int64 // minimum construction value to pass the filter (default: minPermitValueCAD)
}

// NewRichmondCollector returns a RichmondCollector with a 30-second HTTP timeout.
func NewRichmondCollector() *RichmondCollector {
	return &RichmondCollector{
		client:   &http.Client{Timeout: 30 * time.Second},
		MinValue: minPermitValueCAD,
	}
}

// Name satisfies the collector.Collector interface.
func (r *RichmondCollector) Name() string { return "richmond_permits" }

// richmondReportsPerRun is how many of the newest weekly reports each run reads.
// Reading more than one means a missed or failed run does not lose a week;
// permits already seen are dropped by the pipeline's hash dedup.
const richmondReportsPerRun = 3

// Collect satisfies the collector.Collector interface. Downloads the most recent weekly
// building report PDFs, parses all permit records, and returns the relevant ones as RawProjects.
func (r *RichmondCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	urls, err := r.fetchPDFURLs(ctx)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("richmond: no PDF URLs found")
	}
	if len(urls) > richmondReportsPerRun {
		urls = urls[:richmondReportsPerRun] // first link = latest week
	}

	var projects []collector.RawProject
	var failed int
	var firstErr error
	for _, pdfURL := range urls {
		reportProjects, err := r.collectReport(ctx, pdfURL)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			logger.Log.Warn("richmond report failed", "url", pdfURL, "error", err)
			continue
		}
		projects = append(projects, reportProjects...)
	}
	if failed == len(urls) {
		return nil, firstErr
	}
	return projects, nil
}

// collectReport downloads and parses one weekly report and filters its permits.
func (r *RichmondCollector) collectReport(ctx context.Context, pdfURL string) ([]collector.RawProject, error) {
	if r.Verbose {
		logger.Log.Info("processing report", "source", "richmond", "url", pdfURL)
	}

	path, rawData, cleanup, err := r.downloadPDF(ctx, pdfURL)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	records, err := parsePDF(path)
	if err != nil {
		return nil, err
	}

	if r.Verbose {
		logger.Log.Info("parsed records from PDF", "source", "richmond", "count", len(records), "url", pdfURL)
		counts := make(map[string]int)
		for _, rec := range records {
			counts[rec.SubType]++
		}
		for subType, n := range counts {
			logger.Log.Debug("permits by sub-type", "source", "richmond", "sub_type", subType, "count", n)
		}
	}

	var projects []collector.RawProject
	var skippedValue, skippedType int
	for _, rec := range records {
		if rec.ValueCAD <= r.MinValue {
			skippedValue++
			continue
		}
		if !commercialSubTypes[strings.ToLower(strings.TrimSpace(rec.SubType))] {
			skippedType++
			continue
		}
		p := toRawProject(rec, rawData)
		p.SourceURL = pdfURL
		p.Hash = hashPermit(rec.FolderNumber, rec.Address, rec.IssueDate)
		projects = append(projects, p)
	}

	if r.Verbose {
		logger.Log.Info("filtering complete",
			"source", "richmond",
			"passed", len(projects),
			"skipped_low_value", skippedValue,
			"skipped_residential", skippedType,
			"min_value", r.MinValue,
		)
	}

	return projects, nil
}

// fetchPDFURLs scrapes the weekly reports page and returns absolute URLs for
// building report PDFs. Demolition reports are excluded by the regex.
func (r *RichmondCollector) fetchPDFURLs(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, richmondReportsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("richmond: build request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richmond: fetch reports page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("richmond: reports page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("richmond: read response body: %w", err)
	}

	paths := buildingReportRe.FindAllString(string(body), -1)
	if len(paths) == 0 {
		return nil, fmt.Errorf("richmond: no building report PDFs found — page structure may have changed")
	}

	seen := make(map[string]bool)
	urls := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			urls = append(urls, richmondBaseURL+p)
		}
	}
	return urls, nil
}

// downloadPDF fetches a PDF from url, writes it to a temp file, and returns
// the file path, the raw bytes, plus a cleanup function the caller must defer.
func (r *RichmondCollector) downloadPDF(ctx context.Context, url string) (path string, data []byte, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: build pdf request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: download pdf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, nil, fmt.Errorf("richmond: pdf download returned HTTP %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: read pdf body: %w", err)
	}

	tmp, err := os.CreateTemp("", "richmond-*.pdf")
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: create temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, nil, fmt.Errorf("richmond: write pdf: %w", err)
	}
	tmp.Close()

	return tmp.Name(), data, func() { os.Remove(tmp.Name()) }, nil
}

// findPdftotext returns the path to the pdftotext binary.
// Checks the location bundled with Git for Windows first, then falls back to PATH.
func findPdftotext() (string, error) {
	const gitPath = `C:\Program Files\Git\mingw64\bin\pdftotext.exe`
	if _, err := os.Stat(gitPath); err == nil {
		return gitPath, nil
	}
	p, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("richmond: pdftotext not found — install Poppler or Git for Windows (expected at %s)", gitPath)
	}
	return p, nil
}

// parseDollarAmount converts "$300,000.00" → 300000.
func parseDollarAmount(s string) int64 {
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if dot := strings.Index(s, "."); dot != -1 {
		s = s[:dot]
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// minPermitValueCAD is the minimum construction value for a permit to be considered.
// Residential and low-value permits rarely involve out-of-town crews.
const minPermitValueCAD = 500_000

// commercialSubTypes is the whitelist of permit sub-types relevant to hotel group sales.
// Residential sub-types (One Family Dwelling, Townhouse, etc.) are excluded — they
// don't generate construction crew lodging demand at scale.
var commercialSubTypes = map[string]bool{
	"hotel":                   true,
	"warehouse":               true,
	"manufacturing/warehouse": true, // Richmond uses this combined sub-type
	"office":                  true,
	"medical office":          true,
	"dental office":           true,
	"financial institute":     true, // banks, credit unions — large TI projects
	"restaurant":              true,
	"retail":                  true,
	"apartment":               true,
	"educational facility":    true,
	"community hall":          true,
	"recreational":            true,
	"industrial":              true,
	"canopy":                  true,
	"nursing home":            true, // extended renovation projects, multi-trade crews
}

// isRelevant returns true if a permit record is worth enriching.
// Filters out residential sub-types and permits strictly below minValue.
func isRelevant(rec permitRecord, minValue int64) bool {
	if rec.ValueCAD <= minValue {
		return false
	}
	return commercialSubTypes[strings.ToLower(strings.TrimSpace(rec.SubType))]
}

// toRawProject maps a permitRecord to the normalized collector.RawProject used by the pipeline.
func toRawProject(rec permitRecord, rawData []byte) collector.RawProject {
	return collector.RawProject{
		Source:     "richmond_permits",
		ExternalID: rec.FolderNumber,
		Title:      fmt.Sprintf("%s — %s", rec.SubType, rec.Address),
		Location:   rec.Address + ", Richmond BC",
		Value:      rec.ValueCAD,
		Description: fmt.Sprintf(
			"Work: %s | Status: %s | Applicant: %s | Contractor: %s",
			rec.WorkProposed, rec.Status, rec.Applicant, rec.Contractor,
		),
		IssuedAt: rec.IssueDate,
		RawData:  rawData,
		RawType:  "application/pdf",
		Metadata: map[string]any{
			"applicant":  rec.Applicant,
			"contractor": rec.Contractor,
		},
	}
}

// formatCAD formats an int64 dollar amount with comma separators for log output.
func formatCAD(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// hashPermit produces a deterministic dedup key for a Richmond permit.
// Uses folder number (unique per permit) + address + date to guard against
// re-processing the same permit if it appears in multiple weekly reports.
func hashPermit(folderNumber, address string, issuedAt time.Time) string {
	h := sha256.Sum256([]byte(
		"richmond_permits|" + folderNumber + "|" + address + "|" + issuedAt.Format("2006-01-02"),
	))
	return fmt.Sprintf("%x", h)
}
