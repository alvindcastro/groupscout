package collector

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"github.com/alvindcastro/groupscout/internal/logger"
	"golang.org/x/net/html"
	"io"
)

// creativeBCDefaultURL is the Salesforce Visualforce page that server-renders the in-production
// list as static HTML. isdtp=p1 strips Salesforce navigation chrome.
const creativeBCDefaultURL = "https://knowledgehub.creativebc.com/apex/In_Production_List?isdtp=p1"

// creativeBCKeepTypes maps normalized production types to keep (Feature Film + TV Series only).
// Animation, Documentary, New Media, and Mini Series are primarily local-crew work.
var creativeBCKeepTypes = map[string]bool{
	"feature film": true,
	"tv series":    true,
}

// creativeBCRecord holds one parsed entry from the Creative BC in-production list.
// Fields come directly from the labeled <b> elements on the Visualforce page.
type creativeBCRecord struct {
	Title    string
	Type     string // normalized: "Feature Film", "TV Series", etc.
	Studio   string // Local Production Company
	Schedule string // "3/9/2026 - 4/10/2026"
	Address  string // Production Address — used for location scoring
	Manager  string // Production Manager
	Email    string
}

// CreativeBCCollector fetches the Creative BC "In Production" Visualforce page and returns
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
// absent from Go's default root pool on Linux. No credentials are transmitted.
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
// Fetches the Creative BC in-production page, parses all productions, filters to Feature Film
// and TV Series, and returns RawProjects. Errors produce a warning log and an empty slice —
// they do not abort the pipeline run.
func (c *CreativeBCCollector) Collect(ctx context.Context) ([]RawProject, error) {
	body, err := c.fetchHTML(ctx)
	if err != nil {
		logger.Log.Warn("could not fetch creativebc page", "error", err)
		return nil, nil
	}

	records, err := parseCreativeBCHTML(body)
	if err != nil {
		logger.Log.Warn("could not parse creativebc page", "error", err)
		return nil, nil
	}

	if c.Verbose {
		logger.Log.Info("parsed records from creativebc", "count", len(records))
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
		logger.Log.Info("filtering complete", "source", "creativebc", "passed", len(projects))
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

// parseCreativeBCHTML parses the Creative BC in-production Visualforce page.
//
// Page structure (actual HTML from knowledgehub.creativebc.com/apex/In_Production_List):
//
//	<div id="inProductionList">
//	  <table class="detailList">
//	    <tr><td><h5>Feature</h5></td></tr>                   ← category header
//	    <tr><td>
//	      <h3 class="production">FARADAY</h3>                ← title (ALL CAPS)
//	      <b>Local Production Company: </b>ABG Productions   ← labeled fields
//	      <b>Schedule: </b>3/9/2026 - 4/10/2026
//	      <b>Production Address: </b>3920 Norland Ave...
//	    </td></tr>
//	    <tr><td><h5>TV Series</h5></td></tr>
//	    ...
//	  </table>
//	</div>
func parseCreativeBCHTML(body []byte) ([]creativeBCRecord, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creativebc: parse html: %w", err)
	}

	listDiv := findNodeByID(doc, "inProductionList")
	if listDiv == nil {
		return nil, fmt.Errorf("creativebc: production list not found in page")
	}

	var records []creativeBCRecord
	var currentType string
	var current *creativeBCRecord

	flush := func() {
		if current != nil && current.Title != "" {
			records = append(records, *current)
			current = nil
		}
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			return
		}

		switch n.Data {
		case "h5":
			// Category header: "Feature", "TV Series", "Doc Series", etc.
			currentType = normalizeProdType(strings.TrimSpace(nodeText(n)))
			return // don't recurse

		case "h3":
			if hasClass(n, "production") {
				flush()
				current = &creativeBCRecord{
					Title: toTitleCase(strings.TrimSpace(nodeText(n))),
					Type:  currentType,
				}
				return // don't recurse into h3
			}

		case "b":
			if current != nil {
				label := strings.ToLower(strings.TrimRight(strings.TrimSpace(nodeText(n)), ":"))
				// Value is the text node immediately following the <b> closing tag
				var value string
				if sib := n.NextSibling; sib != nil && sib.Type == html.TextNode {
					value = strings.TrimSpace(sib.Data)
				}
				if value != "" {
					switch label {
					case "local production company":
						current.Studio = value
					case "schedule":
						current.Schedule = value
					case "production address":
						current.Address = value
					case "production manager":
						current.Manager = value
					case "email":
						current.Email = value
					}
				}
				return // don't recurse into b
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(listDiv)
	flush()

	if len(records) == 0 {
		return nil, fmt.Errorf("creativebc: no productions found in page")
	}
	return records, nil
}

// normalizeProdType maps the h5 category text to a consistent production type string.
func normalizeProdType(h5text string) string {
	switch strings.ToLower(h5text) {
	case "feature":
		return "Feature Film"
	case "tv series":
		return "TV Series"
	case "doc series":
		return "Documentary Series"
	case "mini series":
		return "Mini Series"
	case "new media feature":
		return "New Media Feature"
	case "new media series":
		return "New Media Series"
	default:
		return h5text
	}
}

// findNodeByID returns the first node with the given id attribute, or nil.
func findNodeByID(root *html.Node, id string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == id {
					found = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// hasClass reports whether an element node has the given CSS class.
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

// nodeText returns all text content within an HTML node, concatenated and trimmed.
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

// toTitleCase converts ALL CAPS production titles to Title Case.
// "QUEENS FOR A DAY" → "Queens For A Day"
func toTitleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// isCreativeBCRelevant returns true if the production type is Feature Film or TV Series.
func isCreativeBCRelevant(rec creativeBCRecord) bool {
	return creativeBCKeepTypes[strings.ToLower(strings.TrimSpace(rec.Type))]
}

// toCreativeBCRawProject maps a creativeBCRecord to the normalized RawProject.
// Address and schedule are included in Description so Claude can use them for location scoring.
func toCreativeBCRawProject(rec creativeBCRecord) RawProject {
	desc := fmt.Sprintf("%s — %s", rec.Type, rec.Studio)
	if rec.Schedule != "" {
		desc += fmt.Sprintf(" | Schedule: %s", rec.Schedule)
	}
	if rec.Address != "" {
		desc += fmt.Sprintf(" | Address: %s", rec.Address)
	}

	location := extractCreativeBCCity(rec.Address)

	return RawProject{
		Source:      "creativebc",
		ExternalID:  slugify(rec.Title),
		Title:       rec.Title,
		Location:    location,
		Value:       0,
		Description: desc,
		IssuedAt:    parseScheduleStart(rec.Schedule),
		RawData: map[string]any{
			"production_type": rec.Type,
			"studio":          rec.Studio,
			"schedule":        rec.Schedule,
			"address":         rec.Address,
			"manager":         rec.Manager,
			"email":           rec.Email,
			"applicant":       rec.Studio,
			"contractor":      "",
		},
	}
}

// extractCreativeBCCity pulls the city name from a production address string.
// Format: "3920 Norland Avenue, Burnaby, Canada, V5G 4K7" → "Burnaby, BC"
func extractCreativeBCCity(address string) string {
	if address == "" {
		return "Metro Vancouver, BC"
	}
	parts := strings.Split(address, ",")
	if len(parts) >= 2 {
		city := strings.TrimSpace(parts[1])
		if city != "" && strings.ToLower(city) != "canada" {
			return city + ", BC"
		}
	}
	return "Metro Vancouver, BC"
}

// parseScheduleStart parses the start date from "M/D/YYYY - M/D/YYYY" schedule strings.
func parseScheduleStart(schedule string) time.Time {
	if schedule == "" {
		return time.Time{}
	}
	parts := strings.SplitN(schedule, " - ", 2)
	t, err := time.Parse("1/2/2006", strings.TrimSpace(parts[0]))
	if err != nil {
		return time.Time{}
	}
	return t
}

// hashCreativeBCProduction produces a deterministic dedup key keyed on title + type.
// Case-insensitive so capitalisation changes don't regenerate the same lead.
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
