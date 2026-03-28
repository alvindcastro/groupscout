package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

const (
	richmondBaseURL    = "https://www.richmond.ca"
	richmondReportsURL = "https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm"
)

// buildingReportRe matches building report PDF href paths on the weekly reports page.
// Example: /__shared/assets/buildingreportmarch15_202678649.pdf
var buildingReportRe = regexp.MustCompile(`/__shared/assets/buildingreport[^"]+\.pdf`)

// folderNumRe identifies the start of a new permit record.
// Folder numbers follow the pattern: 25 036523 000 00 B7
var folderNumRe = regexp.MustCompile(`^\d{2}\s+\d{6}`)

// subTypeRe matches SUB TYPE section headers, e.g. "SUB TYPE: Hotel"
var subTypeRe = regexp.MustCompile(`(?i)SUB\s+TYPE:\s*(.+)`)

// skipLineRe matches lines that are column headers, totals, or page noise to ignore during parsing.
var skipLineRe = regexp.MustCompile(`(?i)^(folder\s*number|work\s*proposed|status|issue\s*date|constr|applicant|contractor|sub\s*total|grand\s*total|building\s*permit|city\s*of\s*richmond)`)

// issueDateRe matches YYYY/MM/DD date strings.
var issueDateRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)

// valueRe matches dollar amounts like $300,000.00
var valueRe = regexp.MustCompile(`^\$[\d,]+\.?\d*$`)

// permitRecord holds the raw fields extracted from one permit entry in a Richmond PDF report.
// The PDF renders each field on its own line in reading order, so records are parsed
// positionally: folder number → work proposed → status → date → value → address → applicant → contractor.
type permitRecord struct {
	SubType      string    // e.g. "Hotel", "Warehouse", "Office"
	FolderNumber string    // e.g. "25 036523 000 00 B7"
	WorkProposed string    // e.g. "New", "Alteration", "Revision"
	Status       string    // e.g. "Issued"
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
	client *http.Client
}

// NewRichmondCollector returns a RichmondCollector with a 30-second HTTP timeout.
func NewRichmondCollector() *RichmondCollector {
	return &RichmondCollector{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name satisfies the Collector interface.
func (r *RichmondCollector) Name() string { return "richmond_permits" }

// Collect satisfies the Collector interface. Downloads the most recent weekly
// building report PDF, parses all permit records, and returns them as RawProjects.
// Filter + mapping logic added in A3.
func (r *RichmondCollector) Collect(ctx context.Context) ([]RawProject, error) {
	urls, err := r.fetchPDFURLs(ctx)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("richmond: no PDF URLs found")
	}

	// Always process only the most recent report (first link = latest week).
	path, cleanup, err := r.downloadPDF(ctx, urls[0])
	if err != nil {
		return nil, err
	}
	defer cleanup()

	records, err := parsePDF(path)
	if err != nil {
		return nil, err
	}

	// A3: filter by sub-type + value, map to RawProject
	_ = records
	return nil, nil
}

// fetchPDFURLs scrapes the weekly reports page and returns absolute URLs for
// building report PDFs. Demolition reports are excluded by the regex.
func (r *RichmondCollector) fetchPDFURLs(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, richmondReportsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("richmond: build request: %w", err)
	}
	req.Header.Set("User-Agent", "blockscout-leadgen/1.0 (hotel group sales intelligence)")

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
// the file path plus a cleanup function the caller must defer.
func (r *RichmondCollector) downloadPDF(ctx context.Context, url string) (path string, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("richmond: build pdf request: %w", err)
	}
	req.Header.Set("User-Agent", "blockscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("richmond: download pdf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("richmond: pdf download returned HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "richmond-*.pdf")
	if err != nil {
		return "", nil, fmt.Errorf("richmond: create temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("richmond: write pdf: %w", err)
	}
	tmp.Close()

	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}

// parsePDF opens a PDF file and extracts all permit records from its text content.
// Each page is read row by row; each row's text fragments are joined into one line.
// Records are then parsed from the flat line list by parsePermitLines.
func parsePDF(path string) ([]permitRecord, error) {
	f, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("richmond: open pdf: %w", err)
	}
	defer f.Close()

	var lines []string
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			var parts []string
			for _, text := range row.Content {
				if s := strings.TrimSpace(text.S); s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				lines = append(lines, strings.Join(parts, " "))
			}
		}
	}

	return parsePermitLines(lines), nil
}

// parsePermitLines converts a flat slice of text lines into permit records.
//
// PDF structure (one permit = 7–8 consecutive lines):
//   line 0: FOLDER NUMBER  (e.g. "25 036523 000 00 B7")
//   line 1: WORK PROPOSED  (e.g. "Alteration")
//   line 2: STATUS         (e.g. "Issued")
//   line 3: ISSUE DATE     (e.g. "2026/03/16")
//   line 4: CONSTR. VALUE  (e.g. "$300,000.00")
//   line 5: ADDRESS        (e.g. "8640 Alexandra Road")
//   line 6: APPLICANT
//   line 7: CONTRACTOR
//
// Section headers ("SUB TYPE: Hotel") reset the current sub-type.
// Column headers, totals, and page noise are skipped via skipLineRe.
func parsePermitLines(lines []string) []permitRecord {
	var records []permitRecord
	var current *permitRecord
	var currentSubType string
	fieldIdx := 0

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Section header — update sub-type, reset current record
		if m := subTypeRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				records = append(records, *current)
				current = nil
			}
			currentSubType = strings.TrimSpace(m[1])
			continue
		}

		// Skip column headers, totals, and page chrome
		if skipLineRe.MatchString(line) {
			continue
		}

		// New permit record starts with a folder number
		if folderNumRe.MatchString(line) {
			if current != nil {
				records = append(records, *current)
			}
			current = &permitRecord{
				SubType:      currentSubType,
				FolderNumber: line,
			}
			fieldIdx = 0
			continue
		}

		if current == nil {
			continue
		}

		// Assign fields positionally after the folder number line
		switch fieldIdx {
		case 0:
			current.WorkProposed = line
		case 1:
			current.Status = line
		case 2:
			if t, err := time.Parse("2006/01/02", line); err == nil {
				current.IssueDate = t
			}
		case 3:
			current.ValueCAD = parseDollarAmount(line)
		case 4:
			current.Address = line
		case 5:
			current.Applicant = line
		case 6:
			current.Contractor = line
		}
		fieldIdx++
	}

	// Flush the last record
	if current != nil {
		records = append(records, *current)
	}

	return records
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
