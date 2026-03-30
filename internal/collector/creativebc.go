package collector

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// creativeBCDefaultURL is the Salesforce Visualforce page that renders the in-production list
// as server-side HTML. The isdtp=p1 parameter strips Salesforce navigation chrome.
// Plain net/http GET works — no JavaScript required.
const creativeBCDefaultURL = "https://knowledgehub.creativebc.com/apex/In_Production_List?isdtp=p1"

// creativeBCKeepTypes are the production types that bring large out-of-town crews.
// Animation, VFX, Documentary, and Short Film are primarily local-crew work.
var creativeBCKeepTypes = map[string]bool{
	"feature film": true,
	"tv series":    true,
}

// creativeBCRecord holds one parsed row from the Creative BC in-production list.
type creativeBCRecord struct {
	Title  string
	Type   string // e.g. "Feature Film", "TV Series"
	Studio string // Studio/Distributor
	Status string // e.g. "Principal Photography", "Pre-Production"
}

// CreativeBCCollector fetches the Creative BC "In Production" HTML page and returns
// Feature Film and TV Series productions as RawProject values.
// Enabled via CREATIVEBC_ENABLED=true; URL overridable via CREATIVEBC_URL.
type CreativeBCCollector struct {
	URL     string
	client  *http.Client
	Verbose bool
}

// NewCreativeBCCollector returns a CreativeBCCollector.
// urlOverride replaces the default Visualforce URL when non-empty.
// TLS verification is skipped: the Knowledge Hub uses a Salesforce intermediate CA
// that is absent from Go's default root pool on Linux. No credentials are transmitted.
func NewCreativeBCCollector(urlOverride string) *CreativeBCCollector {
	u := creativeBCDefaultURL
	if urlOverride != "" {
		u = urlOverride
	}
	return &CreativeBCCollector{
		URL: u,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

// Name satisfies the Collector interface.
func (c *CreativeBCCollector) Name() string { return "creativebc" }

// Collect satisfies the Collector interface.
// Fetches the Creative BC in-production HTML page, parses the production table, filters to
// Feature Film and TV Series, and returns RawProjects. Fetch or parse errors produce a warning
// log and an empty slice — they do not abort the pipeline run.
func (c *CreativeBCCollector) Collect(ctx context.Context) ([]RawProject, error) {
	body, err := c.fetchHTML(ctx)
	if err != nil {
		log.Printf("[creativebc] warning: could not fetch page: %v — skipping", err)
		return nil, nil
	}

	records, err := parseCreativeBCHTML(body)
	if err != nil {
		log.Printf("[creativebc] warning: could not parse page: %v — skipping", err)
		return nil, nil
	}

	if c.Verbose {
		log.Printf("[creativebc] parsed %d production records from HTML", len(records))
	}

	var projects []RawProject
	for _, rec := range records {
		if !isCreativeBCRelevant(rec) {
			continue
		}
		p := toCreativeBCRawProject(rec)
		p.SourceURL = c.URL
		p.Hash = hashCreativeBCProduction(rec.Title, rec.Type)
		projects = append(projects, p)
	}

	if c.Verbose {
		log.Printf("[creativebc] %d productions passed filter (Feature Film + TV Series)", len(projects))
	}

	return projects, nil
}

// fetchHTML fetches the Visualforce page and returns the response body.
func (c *CreativeBCCollector) fetchHTML(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creativebc: build request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")
	req.Header.Set("Accept", "text/html")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creativebc: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("creativebc: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("creativebc: read body: %w", err)
	}
	return body, nil
}

// parseCreativeBCHTML parses the production table from the Creative BC Visualforce page HTML.
//
// Strategy:
//  1. Walk the HTML tree to find all <table> elements.
//  2. For each table, check whether the header row contains "title" and "type" (case-insensitive).
//  3. Map column indices from the header row.
//  4. Extract each data row using those column indices.
func parseCreativeBCHTML(body []byte) ([]creativeBCRecord, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creativebc: parse html: %w", err)
	}

	tables := findTables(doc)
	for _, table := range tables {
		records, ok := extractProductionTable(table)
		if ok {
			return records, nil
		}
	}

	return nil, fmt.Errorf("creativebc: no production table found in page")
}

// findTables returns all <table> nodes in document order.
func findTables(n *html.Node) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "table" {
			tables = append(tables, node)
			return // don't recurse into nested tables
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return tables
}

// extractProductionTable attempts to parse a <table> node as the production list.
// Returns the parsed records and true if this table looks like the right one.
func extractProductionTable(table *html.Node) ([]creativeBCRecord, bool) {
	rows := tableRows(table)
	if len(rows) < 2 {
		return nil, false
	}

	// Find header row — first row whose cells contain "title" and "type"
	headerIdx := -1
	colTitle, colType, colStudio, colStatus := -1, -1, -1, -1

	for i, row := range rows {
		cells := rowCells(row)
		if len(cells) < 2 {
			continue
		}
		// Check if this looks like the header
		titleFound, typeFound := false, false
		for j, cell := range cells {
			text := strings.ToLower(strings.TrimSpace(nodeText(cell)))
			switch {
			case strings.Contains(text, "title"):
				colTitle = j
				titleFound = true
			case strings.Contains(text, "type"):
				colType = j
				typeFound = true
			case strings.Contains(text, "studio") || strings.Contains(text, "distributor"):
				colStudio = j
			case text == "status":
				colStatus = j
			}
		}
		if titleFound && typeFound {
			headerIdx = i
			break
		}
	}

	if headerIdx == -1 || colTitle == -1 || colType == -1 {
		return nil, false
	}

	var records []creativeBCRecord
	for _, row := range rows[headerIdx+1:] {
		cells := rowCells(row)
		if len(cells) == 0 {
			continue
		}
		rec := creativeBCRecord{
			Title:  cellText(cells, colTitle),
			Type:   cellText(cells, colType),
			Studio: cellText(cells, colStudio),
			Status: cellText(cells, colStatus),
		}
		if rec.Title == "" || rec.Type == "" {
			continue
		}
		records = append(records, rec)
	}

	return records, true
}

// tableRows returns all <tr> nodes within a table (from thead, tbody, or directly).
func tableRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			rows = append(rows, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

// rowCells returns all <td> and <th> nodes in a <tr>.
func rowCells(tr *html.Node) []*html.Node {
	var cells []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, c)
		}
	}
	return cells
}

// nodeText returns all text content within an HTML node, concatenated.
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// cellText returns the trimmed text of a cell at the given index, or "" if out of range.
func cellText(cells []*html.Node, idx int) string {
	if idx < 0 || idx >= len(cells) {
		return ""
	}
	return nodeText(cells[idx])
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
			"applicant":       rec.Studio,
			"contractor":      "",
		},
	}
}

// hashCreativeBCProduction produces a deterministic dedup key keyed on title + type.
// Case-insensitive and content-independent so weekly list reorders don't regenerate leads.
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
