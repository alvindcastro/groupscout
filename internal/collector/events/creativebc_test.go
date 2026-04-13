package events

import (
	"testing"
)

// sampleCreativeBCHTML mirrors the actual structure of the Creative BC Visualforce page.
// Productions are grouped under <h5> category headers; each entry uses <h3 class="production">
// for the title and labeled <b> elements for fields.
var sampleCreativeBCHTML = []byte(`<!DOCTYPE html>
<html><body>
<div id="inProductionList">
  <table class="detailList">
    <tr><td class="data2Col first" colSpan="2">
      <h5>Feature</h5>
    </td></tr>
    <tr><td class="data2Col" colSpan="2">
      <span>
        <h3 class="production">FARADAY</h3>
        <span>
          <span><b>Local Production Company: </b>ABG Productions BC Inc</span>
          <p><b>Production Manager: </b>Gabriel Zamora</p>
          <span><b>Schedule: </b>3/9/2026 - 4/10/2026</span>
          <p><b>Production Address: </b>3920 Norland Avenue, Burnaby, Canada, V5G 4K7</p>
          <p><b>Email: </b>abgproductionoffice@gmail.com</p>
        </span>
      </span>
    </td></tr>
    <tr><td class="data2Col" colSpan="2">
      <span>
        <h3 class="production">WHITE ELEPHANT</h3>
        <span>
          <span><b>Local Production Company: </b>Scratch That CAN Productions Inc.</span>
          <span><b>Schedule: </b>3/30/2026 - 5/1/2026</span>
          <p><b>Production Address: </b>18788 96 Avenue, Surrey, Canada, V4N 3R1</p>
        </span>
      </span>
    </td></tr>
    <tr><td class="data2Col first" colSpan="2">
      <h5>TV Series</h5>
    </td></tr>
    <tr><td class="data2Col" colSpan="2">
      <span>
        <h3 class="production">VANCOUVER CHRONICLES - SEASON 2</h3>
        <span>
          <span><b>Local Production Company: </b>BC Productions Ltd</span>
          <p><b>Production Manager: </b>Jane Smith</p>
          <span><b>Schedule: </b>2/1/2026 - 8/31/2026</span>
          <p><b>Production Address: </b>123 Main St, Vancouver, Canada, V6B 1A1</p>
        </span>
      </span>
    </td></tr>
    <tr><td class="data2Col first" colSpan="2">
      <h5>Doc Series</h5>
    </td></tr>
    <tr><td class="data2Col" colSpan="2">
      <span>
        <h3 class="production">NATURE DOCS</h3>
        <span>
          <span><b>Local Production Company: </b>Doc Films Inc</span>
          <span><b>Schedule: </b>1/1/2026 - 3/31/2026</span>
        </span>
      </span>
    </td></tr>
    <tr><td class="data2Col first" colSpan="2">
      <h5>New Media Series</h5>
    </td></tr>
    <tr><td class="data2Col" colSpan="2">
      <span>
        <h3 class="production">ANAHEIM REALM - SEASON 1</h3>
        <span>
          <span><b>Local Production Company: </b>Pico Productions (BC) Limited</span>
          <span><b>Schedule: </b>2/25/2026 - 4/1/2027</span>
        </span>
      </span>
    </td></tr>
  </table>
</div>
</body></html>`)

func TestParseCreativeBCHTML_count(t *testing.T) {
	records, err := parseCreativeBCHTML(sampleCreativeBCHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}
}

func TestParseCreativeBCHTML_firstRecord(t *testing.T) {
	records, err := parseCreativeBCHTML(sampleCreativeBCHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := records[0]

	if r.Title != "Faraday" {
		t.Errorf("Title: got %q, want %q", r.Title, "Faraday")
	}
	if r.Type != "Feature Film" {
		t.Errorf("Type: got %q, want %q", r.Type, "Feature Film")
	}
	if r.Studio != "ABG Productions BC Inc" {
		t.Errorf("Studio: got %q, want %q", r.Studio, "ABG Productions BC Inc")
	}
	if r.Schedule != "3/9/2026 - 4/10/2026" {
		t.Errorf("Schedule: got %q, want %q", r.Schedule, "3/9/2026 - 4/10/2026")
	}
	if r.Address != "3920 Norland Avenue, Burnaby, Canada, V5G 4K7" {
		t.Errorf("Address: got %q, want %q", r.Address, "3920 Norland Avenue, Burnaby, Canada, V5G 4K7")
	}
	if r.Manager != "Gabriel Zamora" {
		t.Errorf("Manager: got %q, want %q", r.Manager, "Gabriel Zamora")
	}
}

func TestParseCreativeBCHTML_tvSeries(t *testing.T) {
	records, err := parseCreativeBCHTML(sampleCreativeBCHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := records[2] // third record, first TV Series
	if r.Type != "TV Series" {
		t.Errorf("Type: got %q, want %q", r.Type, "TV Series")
	}
	if r.Title != "Vancouver Chronicles - Season 2" {
		t.Errorf("Title: got %q, want %q", r.Title, "Vancouver Chronicles - Season 2")
	}
}

func TestParseCreativeBCHTML_noList(t *testing.T) {
	_, err := parseCreativeBCHTML([]byte(`<html><body><p>No list here</p></body></html>`))
	if err == nil {
		t.Error("expected error when inProductionList div not found")
	}
}

func TestNormalizeProdType(t *testing.T) {
	cases := []struct{ input, want string }{
		{"Feature", "Feature Film"},
		{"TV Series", "TV Series"},
		{"Doc Series", "Documentary Series"},
		{"Mini Series", "Mini Series"},
		{"New Media Feature", "New Media Feature"},
		{"New Media Series", "New Media Series"},
	}
	for _, c := range cases {
		got := normalizeProdType(c.input)
		if got != c.want {
			t.Errorf("normalizeProdType(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestIsCreativeBCRelevant_featureFilm(t *testing.T) {
	if !isCreativeBCRelevant(creativeBCRecord{Type: "Feature Film"}) {
		t.Error("Feature Film should pass filter")
	}
}

func TestIsCreativeBCRelevant_tvSeries(t *testing.T) {
	if !isCreativeBCRelevant(creativeBCRecord{Type: "TV Series"}) {
		t.Error("TV Series should pass filter")
	}
}

func TestIsCreativeBCRelevant_documentary(t *testing.T) {
	if isCreativeBCRelevant(creativeBCRecord{Type: "Documentary Series"}) {
		t.Error("Documentary Series should be filtered out")
	}
}

func TestIsCreativeBCRelevant_newMedia(t *testing.T) {
	if isCreativeBCRelevant(creativeBCRecord{Type: "New Media Series"}) {
		t.Error("New Media Series should be filtered out")
	}
}

func TestToCreativeBCRawProject_fields(t *testing.T) {
	rec := creativeBCRecord{
		Title:    "Faraday",
		Type:     "Feature Film",
		Studio:   "ABG Productions BC Inc",
		Schedule: "3/9/2026 - 4/10/2026",
		Address:  "3920 Norland Avenue, Burnaby, Canada, V5G 4K7",
		Manager:  "Gabriel Zamora",
	}
	rawData := []byte("fake html content")
	p := toCreativeBCRawProject(rec, rawData)

	if p.Source != "creativebc" {
		t.Errorf("Source: got %q, want %q", p.Source, "creativebc")
	}
	if p.ExternalID != "faraday" {
		t.Errorf("ExternalID: got %q, want %q", p.ExternalID, "faraday")
	}
	if p.Location != "Burnaby, BC" {
		t.Errorf("Location: got %q, want %q", p.Location, "Burnaby, BC")
	}
	if p.IssuedAt.IsZero() {
		t.Error("IssuedAt should be parsed from schedule start date")
	}
	if string(p.RawData) != "fake html content" {
		t.Errorf("RawData mismatch")
	}
	if p.RawType != "text/html" {
		t.Errorf("RawType mismatch: got %q, want %q", p.RawType, "text/html")
	}
}

func TestExtractCreativeBCCity(t *testing.T) {
	cases := []struct{ input, want string }{
		{"3920 Norland Avenue, Burnaby, Canada, V5G 4K7", "Burnaby, BC"},
		{"18788 96 Avenue, Surrey, Canada, V4N 3R1", "Surrey, BC"},
		{"123 Main St, Vancouver, Canada, V6B 1A1", "Vancouver, BC"},
		{"", "Metro Vancouver, BC"},
	}
	for _, c := range cases {
		got := extractCreativeBCCity(c.input)
		if got != c.want {
			t.Errorf("extractCreativeBCCity(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseScheduleStart(t *testing.T) {
	t1 := parseScheduleStart("3/9/2026 - 4/10/2026")
	if t1.IsZero() {
		t.Error("expected non-zero time for valid schedule")
	}
	if t1.Month() != 3 || t1.Day() != 9 || t1.Year() != 2026 {
		t.Errorf("unexpected date: %v", t1)
	}

	t2 := parseScheduleStart("")
	if !t2.IsZero() {
		t.Error("expected zero time for empty schedule")
	}
}

func TestHashCreativeBCProduction_deterministic(t *testing.T) {
	h1 := hashCreativeBCProduction("Faraday", "Feature Film")
	h2 := hashCreativeBCProduction("Faraday", "Feature Film")
	if h1 != h2 {
		t.Error("hash is not deterministic")
	}
}

func TestHashCreativeBCProduction_caseInsensitive(t *testing.T) {
	h1 := hashCreativeBCProduction("Faraday", "Feature Film")
	h2 := hashCreativeBCProduction("FARADAY", "feature film")
	if h1 != h2 {
		t.Error("hash should be case-insensitive")
	}
}

func TestHashCreativeBCProduction_collision(t *testing.T) {
	h1 := hashCreativeBCProduction("Faraday", "Feature Film")
	h2 := hashCreativeBCProduction("White Elephant", "Feature Film")
	if h1 == h2 {
		t.Error("different productions should produce different hashes")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"The Lost Highway", "the-lost-highway"},
		{"  Vancouver Chronicles  ", "vancouver-chronicles"},
		{"A24 Film: Test!", "a24-film-test"},
		{"Mountain & Light", "mountain-light"},
		{"CAPS TITLE", "caps-title"},
	}
	for _, c := range cases {
		got := slugify(c.input)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestToTitleCase(t *testing.T) {
	cases := []struct{ input, want string }{
		{"FARADAY", "Faraday"},
		{"QUEENS FOR A DAY", "Queens For A Day"},
		{"WHITE ELEPHANT", "White Elephant"},
		{"VANCOUVER CHRONICLES - SEASON 2", "Vancouver Chronicles - Season 2"},
	}
	for _, c := range cases {
		got := toTitleCase(c.input)
		if got != c.want {
			t.Errorf("toTitleCase(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
