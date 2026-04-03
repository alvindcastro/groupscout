package collector

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
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
// Note: "applicant" and "contractor" are intentionally excluded — they are handled separately
// as right-column contact block markers, not skipped.
var skipLineRe = regexp.MustCompile(`(?i)^(folder\s*number|work\s*proposed|status|issue\s*date|constr|sub\s*total|grand\s*total|building\s*permit|city\s*of\s*richmond|filters|issued\s*from)`)

// issueDateRe matches YYYY/MM/DD date strings.
var issueDateRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)

// issueDateHeaderRe matches "ISSUE DATE 2026/03/18" — a date embedded on the ISSUE DATE header
// line. Richmond PDFs for multi-permit sections output the first permit's date on the same line
// as the ISSUE DATE column header. Must be checked before skipLineRe, which discards the whole line.
var issueDateHeaderRe = regexp.MustCompile(`(?i)^issue\s+date\s+(\d{4}/\d{2}/\d{2})$`)

// issueDateWithCountRe matches "2026/03/18 2" — a date followed by a whitespace-separated permit
// count. Richmond PDFs for multi-permit sections print subsequent dates merged with the section
// count on the same line (e.g. "2026/03/18 2" means date=2026/03/18, count=2 permits in section).
var issueDateWithCountRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2})\s+\d+$`)

// valueRe matches dollar amounts like $300,000.00
var valueRe = regexp.MustCompile(`^\$[\d,]+\.?\d*$`)

// folderSuffixRe matches the trailing type code of a Richmond folder number (e.g. "B7").
// pdftotext sometimes wraps the folder number across two lines; this code appears alone on the second line.
var folderSuffixRe = regexp.MustCompile(`^[A-Z]\d+$`)

// folderNumExtractRe matches a complete folder number within a line.
// Used to split "19 878924 000 02 B7 Special Inspection Issued" into number + rest.
var folderNumExtractRe = regexp.MustCompile(`\d{2}\s+\d{6}\s+\d{3}\s+\d{2}\s+[A-Z]\d+`)

// folderNameHeaderRe matches "FOLDER NAME" as a standalone header line.
var folderNameHeaderRe = regexp.MustCompile(`(?i)^folder\s+name\s*$`)

// folderNameDataRe matches "FOLDER NAME 8640 Alexandra Road" — label + address on one line.
var folderNameDataRe = regexp.MustCompile(`(?i)^folder\s+name\s+(.+)`)

// permitCountRe matches a lone integer printed as a row count in the PDF (e.g. "1").
var permitCountRe = regexp.MustCompile(`^\d+$`)

// applicantLineRe matches the APPLICANT right-column label (with optional inline name).
// Richmond PDFs render contact info in a separate right column; pdftotext outputs all
// permit records (left column) first, then all APPLICANT/CONTRACTOR blocks (right column).
var applicantLineRe = regexp.MustCompile(`(?i)^APPLICANT\s*(.*)`)

// contractorLineRe matches the CONTRACTOR right-column label (with optional inline name).
var contractorLineRe = regexp.MustCompile(`(?i)^CONTRACTOR\s*(.*)`)

// permitRecord holds the raw fields extracted from one permit entry in a Richmond PDF report.
// Fields are detected by content (date format, dollar sign, FOLDER NAME prefix, etc.)
// rather than position — the actual pdftotext output includes extra lines (permit count
// integers, column header repeats) that make positional parsing unreliable.
// SectionIndex is used internally to associate the right-column APPLICANT/CONTRACTOR
// contact blocks (which appear after all permit records on each page) with the correct permit.
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
	SectionIndex int // internal: index of the SUB TYPE section this permit belongs to
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
	if r.Verbose {
		log.Printf("[richmond] found %d PDF URLs, using: %s", len(urls), urls[0])
	}

	path, cleanup, err := r.downloadPDF(ctx, urls[0])
	if err != nil {
		return nil, err
	}
	defer cleanup()

	records, err := parsePDF(path)
	if err != nil {
		return nil, err
	}

	if r.Verbose {
		log.Printf("[richmond] parsed %d raw permit records from PDF", len(records))
		counts := make(map[string]int)
		for _, rec := range records {
			counts[rec.SubType]++
		}
		for subType, n := range counts {
			log.Printf("[richmond]   sub-type %-30q  %d permits", subType, n)
		}
	}

	pdfURL := urls[0]
	var projects []RawProject
	for _, rec := range records {
		if !isRelevant(rec, r.MinValue) {
			continue
		}
		p := toRawProject(rec)
		p.SourceURL = pdfURL
		p.Hash = hashPermit(rec.FolderNumber, rec.Address, rec.IssueDate)
		projects = append(projects, p)
	}

	if r.Verbose {
		log.Printf("[richmond] %d permits passed filter (commercial + value > $%s CAD)", len(projects), formatCAD(r.MinValue))
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

// parsePDF extracts all permit records from a Richmond PDF report.
// It shells out to pdftotext (Poppler), which correctly handles Richmond's PDF font encoding.
// The plain-text output is split into lines and fed to parsePermitLines.
func parsePDF(path string) ([]permitRecord, error) {
	pdftotext, err := findPdftotext()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(pdftotext, path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("richmond: pdftotext: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}

	return parsePermitLines(lines), nil
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

// sectionContact holds the raw applicant and contractor strings for one SUB TYPE section.
type sectionContact struct {
	applicant  string
	contractor string
}

// parsePermitLines converts a flat slice of text lines into permit records.
//
// Richmond PDFs are two-column tables. pdftotext (without -layout) reads the left column
// first (all permit records) then the right column (all APPLICANT/CONTRACTOR blocks), so
// contact info for a permit appears well after the permit's own lines. Each APPLICANT block
// corresponds to one SUB TYPE section, in the same order as those sections.
//
// This parser uses a two-phase approach:
//  1. Permit phase: parse permit fields by content type, tagging each record with its
//     section index (increments at each SUB TYPE header).
//  2. Contact phase: triggered by the first APPLICANT line on a page; collect one
//     applicant/contractor block per section in order.
//  3. Zip: after all lines are processed, associate contacts[sectionIndex] with each permit.
//
// Phase transitions:
//   - Permit phase → Contact phase: first APPLICANT line encountered
//   - Contact phase → Permit phase: SUB TYPE header encountered (next page beginning)
func parsePermitLines(lines []string) []permitRecord {
	var records []permitRecord
	var current *permitRecord
	var currentSubType string
	var nextLineIsAddress bool

	var sectionIdx int = -1                  // increments at each SUB TYPE header (permit phase)
	var contactIdx int                       // increments per saved contact block (contact phase)
	contacts := make(map[int]sectionContact) // keyed by contactIdx, which mirrors sectionIdx 0,1,2...
	var curContact sectionContact
	var inContacts bool // true = currently parsing right-column contact blocks
	var pendingApplicant bool
	var pendingContractor bool

	flush := func() {
		if current != nil {
			records = append(records, *current)
			current = nil
		}
	}

	saveContact := func() {
		contacts[contactIdx] = curContact
		contactIdx++
		curContact = sectionContact{}
		pendingApplicant = false
		pendingContractor = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// SUB TYPE header — handled in both phases
		if m := subTypeRe.FindStringSubmatch(line); m != nil {
			if inContacts {
				// Page break: save the last contact block, return to permit phase
				saveContact()
				inContacts = false
			} else {
				flush()
			}
			sectionIdx++
			currentSubType = strings.TrimSpace(m[1])
			nextLineIsAddress = false
			continue
		}

		// APPLICANT label — first occurrence triggers switch to contact phase
		if m := applicantLineRe.FindStringSubmatch(line); m != nil {
			if !inContacts {
				flush()
				inContacts = true
			} else {
				// New APPLICANT = new section's contact block
				saveContact()
			}
			if val := strings.TrimSpace(m[1]); val != "" {
				curContact.applicant = val
				pendingApplicant = false
			} else {
				pendingApplicant = true
			}
			pendingContractor = false
			continue
		}

		// CONTRACTOR label (only meaningful in contact phase)
		if m := contractorLineRe.FindStringSubmatch(line); m != nil {
			if inContacts {
				if val := strings.TrimSpace(m[1]); val != "" {
					curContact.contractor = val
					pendingContractor = false
				} else {
					pendingContractor = true
				}
				pendingApplicant = false
			}
			continue
		}

		// Contact phase: fill pending fields; ignore all other lines
		if inContacts {
			if pendingApplicant {
				curContact.applicant = line
				pendingApplicant = false
			} else if pendingContractor {
				curContact.contractor = line
				pendingContractor = false
			}
			continue
		}

		// ── Permit field parsing (phase 1) ───────────────────────────────────────

		// "ISSUE DATE 2026/03/18" — date embedded on the ISSUE DATE header line.
		// Richmond PDFs for multi-permit sections merge the first permit's date onto the
		// ISSUE DATE column header. Must be checked before skipLineRe which would discard it.
		if m := issueDateHeaderRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				if t, err := time.Parse("2006/01/02", m[1]); err == nil {
					current.IssueDate = t
				}
			}
			continue
		}

		// Skip column headers, totals, and page chrome
		if skipLineRe.MatchString(line) {
			continue
		}

		// New permit record starts with a folder number
		if folderNumRe.MatchString(line) {
			flush()
			fn := line
			if m := folderNumExtractRe.FindString(line); m != "" {
				fn = m
			}
			current = &permitRecord{
				SubType:      currentSubType,
				FolderNumber: fn,
				SectionIndex: sectionIdx,
			}
			nextLineIsAddress = false
			continue
		}

		if current == nil {
			continue
		}

		// pdftotext sometimes wraps the trailing type code (e.g. "B7") to its own line.
		// Accept only when no text fields have been assigned yet.
		if current.WorkProposed == "" && folderSuffixRe.MatchString(line) {
			current.FolderNumber = strings.TrimSpace(current.FolderNumber + " " + line)
			continue
		}

		// Address continuation from "FOLDER NAME" header on previous line
		if nextLineIsAddress {
			current.Address = line
			nextLineIsAddress = false
			continue
		}

		// "FOLDER NAME 8640 Alexandra Road" — address embedded on same line
		if m := folderNameDataRe.FindStringSubmatch(line); m != nil {
			current.Address = strings.TrimSpace(m[1])
			continue
		}

		// "FOLDER NAME" alone — address is on the next line
		if folderNameHeaderRe.MatchString(line) {
			nextLineIsAddress = true
			continue
		}

		// Date line — "2026/03/18" standalone, or "2026/03/18 2" with a trailing permit count.
		// "last date wins": for multi-permit sections pdftotext outputs all dates after all permit
		// records; the permit still in current is the last one in the section, so we overwrite
		// rather than guard with IsZero to ensure each permit ends up with its own correct date.
		dateToParse := line
		if m := issueDateWithCountRe.FindStringSubmatch(line); m != nil {
			dateToParse = m[1]
		}
		if issueDateRe.MatchString(dateToParse) {
			if t, err := time.Parse("2006/01/02", dateToParse); err == nil {
				current.IssueDate = t
			}
			continue
		}

		// Dollar value — first occurrence is construction value; subsequent are subtotals
		if valueRe.MatchString(line) {
			if current.ValueCAD == 0 {
				current.ValueCAD = parseDollarAmount(line)
			}
			continue
		}

		// Lone integer — permit count row printed by pdftotext, skip
		if permitCountRe.MatchString(line) {
			continue
		}

		// Sequential text assignment; already-set fields (e.g. Address via FOLDER NAME) are skipped
		switch {
		case current.WorkProposed == "":
			current.WorkProposed = line
		case current.Status == "":
			current.Status = line
		case current.Address == "":
			current.Address = line
		}
	}

	flush()
	if inContacts {
		saveContact() // save the last page's final contact block
	}

	// Zip contacts onto permits by section index
	for i := range records {
		si := records[i].SectionIndex
		if c, ok := contacts[si]; ok {
			records[i].Applicant = c.applicant
			records[i].Contractor = c.contractor
		}
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

// toRawProject maps a permitRecord to the normalized RawProject used by the pipeline.
func toRawProject(rec permitRecord) RawProject {
	return RawProject{
		Source:     "richmond_permits",
		ExternalID: rec.FolderNumber,
		Title:      fmt.Sprintf("%s — %s", rec.SubType, rec.Address),
		Location:   rec.Address,
		Value:      rec.ValueCAD,
		Description: fmt.Sprintf(
			"Work: %s | Status: %s | Applicant: %s | Contractor: %s",
			rec.WorkProposed, rec.Status, rec.Applicant, rec.Contractor,
		),
		IssuedAt: rec.IssueDate,
		RawData: map[string]any{
			"folder_number": rec.FolderNumber,
			"sub_type":      rec.SubType,
			"work_proposed": rec.WorkProposed,
			"status":        rec.Status,
			"issue_date":    rec.IssueDate.Format("2006-01-02"),
			"value_cad":     rec.ValueCAD,
			"address":       rec.Address,
			"applicant":     rec.Applicant,
			"contractor":    rec.Contractor,
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
