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
	"strings"
	"time"
)

const (
	// creativeBCDefaultURL is the Knowledge Hub redirect that serves the in-production PDF.
	creativeBCDefaultURL = "https://knowledgehub.creativebc.com/s/in-production-list"
)

// creativeBCKeepTypes are the production types that bring large out-of-town crews.
// Animation, VFX, Documentary, and Short Film productions are primarily local-crew work.
var creativeBCKeepTypes = map[string]bool{
	"feature film": true,
	"tv series":    true,
}

// creativeBCKnownTypes is the ordered list of known production type strings.
// Used as anchors in the fallback parser when column detection fails.
var creativeBCKnownTypes = []string{
	"Feature Film",
	"TV Series",
	"Animation Series",
	"Animated Series",
	"Documentary Series",
	"Documentary Film",
	"Animation Film",
	"Short Film",
	"Mini-Series",
	"Mini Series",
	"Web Series",
	"MOW",
}

// creativeBCRecord holds one parsed row from the Creative BC in-production list.
type creativeBCRecord struct {
	Title  string
	Type   string // e.g. "Feature Film", "TV Series"
	Studio string // Studio/Distributor
	Status string // e.g. "Principal Photography", "Pre-Production"
}

// CreativeBCCollector fetches the Creative BC "In Production" PDF and returns
// Feature Film and TV Series productions as RawProject values.
// If the PDF is unavailable, Collect logs a warning and returns nil — it does not abort the pipeline.
type CreativeBCCollector struct {
	PDFURL  string
	client  *http.Client
	Verbose bool
}

// NewCreativeBCCollector returns a CreativeBCCollector pointing at the default Knowledge Hub URL.
func NewCreativeBCCollector() *CreativeBCCollector {
	return &CreativeBCCollector{
		PDFURL: creativeBCDefaultURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name satisfies the Collector interface.
func (c *CreativeBCCollector) Name() string { return "creativebc" }

// Collect satisfies the Collector interface.
// Downloads the Creative BC in-production PDF, parses it, and returns filtered RawProjects.
// Errors downloading or parsing the PDF produce a warning log and an empty slice (not a hard error)
// so that one unavailable source never aborts the full pipeline run.
func (c *CreativeBCCollector) Collect(ctx context.Context) ([]RawProject, error) {
	path, sourceURL, cleanup, err := c.downloadPDF(ctx)
	if err != nil {
		log.Printf("[creativebc] warning: could not fetch PDF: %v — skipping", err)
		return nil, nil
	}
	defer cleanup()

	records, err := parseCreativeBCPDF(path)
	if err != nil {
		log.Printf("[creativebc] warning: could not parse PDF: %v — skipping", err)
		return nil, nil
	}

	if c.Verbose {
		log.Printf("[creativebc] parsed %d production records from PDF", len(records))
	}

	var projects []RawProject
	for _, rec := range records {
		if !isCreativeBCRelevant(rec) {
			continue
		}
		p := toCreativeBCRawProject(rec)
		p.SourceURL = sourceURL
		p.Hash = hashCreativeBCProduction(rec.Title, rec.Type)
		projects = append(projects, p)
	}

	if c.Verbose {
		log.Printf("[creativebc] %d productions passed filter (Feature Film + TV Series)", len(projects))
	}

	return projects, nil
}

// downloadPDF follows the knowledge hub redirect and saves the PDF to a temp file.
// Returns the temp path, the final resolved URL, a cleanup func, and any error.
// Returns a non-nil error if the endpoint does not serve a PDF.
func (c *CreativeBCCollector) downloadPDF(ctx context.Context) (path, resolvedURL string, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.PDFURL, nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("creativebc: build request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", nil, fmt.Errorf("creativebc: download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("creativebc: HTTP %d from %s", resp.StatusCode, c.PDFURL)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/pdf") && !strings.Contains(ct, "octet-stream") {
		return "", "", nil, fmt.Errorf("creativebc: unexpected content-type %q — PDF not available at this URL", ct)
	}

	tmp, err := os.CreateTemp("", "creativebc-*.pdf")
	if err != nil {
		return "", "", nil, fmt.Errorf("creativebc: create temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", "", nil, fmt.Errorf("creativebc: write pdf: %w", err)
	}
	tmp.Close()

	finalURL := c.PDFURL
	if resp.Request != nil && resp.Request.URL != nil {
		if u := resp.Request.URL.String(); u != "" {
			finalURL = u
		}
	}

	return tmp.Name(), finalURL, func() { os.Remove(tmp.Name()) }, nil
}

// parseCreativeBCPDF extracts production records from the Creative BC in-production PDF.
// Uses pdftotext -layout to preserve column alignment.
func parseCreativeBCPDF(path string) ([]creativeBCRecord, error) {
	pdftotext, err := findPdftotext()
	if err != nil {
		return nil, fmt.Errorf("creativebc: %w", err)
	}

	out, err := exec.Command(pdftotext, "-layout", path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("creativebc: pdftotext: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return parseCreativeBCLines(lines), nil
}

// creativeBCHeaderRe detects the table header line by looking for "production title" and "type".
var creativeBCHeaderRe = regexp.MustCompile(`(?i)production\s+title.+type`)

// creativeBCPageChromeRe matches lines that are page decoration rather than data rows.
var creativeBCPageChromeRe = regexp.MustCompile(
	`(?i)^\s*(\d+\s*$|page\s+\d+|in\s+production|bc\s+film|creative\s*bc|production\s+title|as\s+of\s+)`,
)

// parseCreativeBCLines converts pdftotext -layout output into creativeBCRecord values.
//
// Strategy:
//  1. Scan for the header line to determine character-offset positions for each column.
//  2. Parse data rows using those offsets to extract Title / Type / Studio / Status.
//  3. If header detection fails, fall back to type-anchor splitting.
func parseCreativeBCLines(lines []string) []creativeBCRecord {
	headerIdx := -1
	typeCol := -1
	studioCol := -1
	statusCol := -1

	for i, line := range lines {
		if creativeBCHeaderRe.MatchString(line) {
			headerIdx = i
			lower := strings.ToLower(line)

			// "Production Type" or just "Type"
			for _, label := range []string{"production type", "type"} {
				if idx := strings.Index(lower, label); idx != -1 {
					typeCol = idx
					break
				}
			}
			// "Studio/Distributor" or variants
			for _, label := range []string{"studio/distributor", "studio / distributor", "distributor", "studio"} {
				if idx := strings.Index(lower, label); idx != -1 && (typeCol == -1 || idx > typeCol) {
					studioCol = idx
					break
				}
			}
			if idx := strings.Index(lower, "status"); idx != -1 {
				statusCol = idx
			}
			break
		}
	}

	if headerIdx == -1 || typeCol == -1 {
		return parseCreativeBCByTypeAnchor(lines)
	}

	var records []creativeBCRecord
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" || creativeBCPageChromeRe.MatchString(line) {
			continue
		}
		rec := extractCreativeBCRow(line, typeCol, studioCol, statusCol)
		if rec.Title == "" || rec.Type == "" {
			continue
		}
		records = append(records, rec)
	}
	return records
}

// extractCreativeBCRow slices one data line into fields using pre-determined column offsets.
func extractCreativeBCRow(line string, typeCol, studioCol, statusCol int) creativeBCRecord {
	runes := []rune(line)
	n := len(runes)

	colStr := func(start, end int) string {
		if start < 0 || start >= n {
			return ""
		}
		if end <= 0 || end > n {
			end = n
		}
		return strings.TrimSpace(string(runes[start:end]))
	}

	title := colStr(0, typeCol)

	var typ, studio, status string
	if studioCol > typeCol {
		typ = colStr(typeCol, studioCol)
	} else {
		typ = colStr(typeCol, n)
	}

	if studioCol > 0 {
		if statusCol > studioCol {
			studio = colStr(studioCol, statusCol)
		} else {
			studio = colStr(studioCol, n)
		}
	}

	if statusCol > 0 {
		status = colStr(statusCol, n)
	}

	return creativeBCRecord{Title: title, Type: typ, Studio: studio, Status: status}
}

// parseCreativeBCByTypeAnchor is the fallback parser when the header line cannot be found.
// It anchors on known production type strings within each line.
func parseCreativeBCByTypeAnchor(lines []string) []creativeBCRecord {
	var records []creativeBCRecord
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || creativeBCPageChromeRe.MatchString(line) {
			continue
		}
		for _, pt := range creativeBCKnownTypes {
			idx := strings.Index(strings.ToLower(line), strings.ToLower(pt))
			if idx == -1 {
				continue
			}
			title := strings.TrimSpace(line[:idx])
			if title == "" {
				continue
			}
			rest := strings.TrimSpace(line[idx+len(pt):])
			studio, status := splitStudioStatus(rest)
			records = append(records, creativeBCRecord{
				Title:  title,
				Type:   pt,
				Studio: studio,
				Status: status,
			})
			break
		}
	}
	return records
}

// splitStudioStatus separates a trailing "studio status" string into its two parts.
// Status keywords are checked right-to-left so the longest match wins.
var creativeBCStatusKeywords = []string{
	"Principal Photography",
	"Pre-Production",
	"Post Production",
	"Post-Production",
	"Wrap",
	"Hiatus",
}

func splitStudioStatus(s string) (studio, status string) {
	for _, kw := range creativeBCStatusKeywords {
		if idx := strings.Index(s, kw); idx != -1 {
			return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx:])
		}
	}
	return strings.TrimSpace(s), ""
}

// isCreativeBCRelevant returns true if the production type is Feature Film or TV Series.
func isCreativeBCRelevant(rec creativeBCRecord) bool {
	return creativeBCKeepTypes[strings.ToLower(strings.TrimSpace(rec.Type))]
}

// toCreativeBCRawProject maps a creativeBCRecord to the normalized RawProject.
func toCreativeBCRawProject(rec creativeBCRecord) RawProject {
	desc := fmt.Sprintf("%s — %s", rec.Type, rec.Studio)
	if rec.Status != "" {
		desc += fmt.Sprintf(" (%s)", rec.Status)
	}
	return RawProject{
		Source:      "creativebc",
		ExternalID:  slugify(rec.Title),
		Title:       rec.Title,
		Location:    "Metro Vancouver, BC",
		Value:       0,
		Description: desc,
		IssuedAt:    time.Time{},
		RawData: map[string]any{
			"production_type": rec.Type,
			"studio":          rec.Studio,
			"status":          rec.Status,
			"applicant":       rec.Studio, // Studio maps to applicant slot on the Slack card
			"contractor":      "",
		},
	}
}

// hashCreativeBCProduction produces a deterministic dedup key keyed on title + type.
// Content-independent so weekly list position shifts do not generate duplicate leads.
func hashCreativeBCProduction(title, productionType string) string {
	h := sha256.Sum256([]byte(
		"creativebc|" + strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(productionType)),
	))
	return fmt.Sprintf("%x", h)
}

// slugify converts a production title into a URL-safe external ID.
// "The Lost Highway" → "the-lost-highway"
var slugNonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugNonAlphanumRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
