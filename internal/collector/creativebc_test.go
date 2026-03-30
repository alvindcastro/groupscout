package collector

import (
	"testing"
)

// sampleCreativeBCLines is a realistic excerpt from pdftotext -layout output on the
// Creative BC in-production PDF. Column positions are preserved with spaces.
var sampleCreativeBCLines = []string{
	`Creative BC — In Production List                                                   As of March 28, 2026`,
	``,
	`Production Title                       Production Type         Studio/Distributor             Status`,
	``,
	`The Lost Highway                       Feature Film            Netflix                        Principal Photography`,
	`Vancouver Chronicles                   TV Series               CBC                            Pre-Production`,
	`Mountain Light                         Feature Film            A24                            Pre-Production`,
	`Metro Nights                           TV Series               Amazon Studios                 Principal Photography`,
	`Burnaby Blocks                         Animation Series        DHX Media                      Production`,
	`Northern Exposure Doc                  Documentary Series      National Geographic            Post Production`,
	``,
	`1`,
}

func TestParseCreativeBCLines_count(t *testing.T) {
	records := parseCreativeBCLines(sampleCreativeBCLines)
	if len(records) != 6 {
		t.Fatalf("expected 6 records, got %d", len(records))
	}
}

func TestParseCreativeBCLines_firstRecord(t *testing.T) {
	records := parseCreativeBCLines(sampleCreativeBCLines)
	r := records[0]

	if r.Title != "The Lost Highway" {
		t.Errorf("Title: got %q, want %q", r.Title, "The Lost Highway")
	}
	if r.Type != "Feature Film" {
		t.Errorf("Type: got %q, want %q", r.Type, "Feature Film")
	}
	if r.Studio != "Netflix" {
		t.Errorf("Studio: got %q, want %q", r.Studio, "Netflix")
	}
	if r.Status != "Principal Photography" {
		t.Errorf("Status: got %q, want %q", r.Status, "Principal Photography")
	}
}

func TestParseCreativeBCLines_secondRecord(t *testing.T) {
	records := parseCreativeBCLines(sampleCreativeBCLines)
	r := records[1]

	if r.Title != "Vancouver Chronicles" {
		t.Errorf("Title: got %q, want %q", r.Title, "Vancouver Chronicles")
	}
	if r.Type != "TV Series" {
		t.Errorf("Type: got %q, want %q", r.Type, "TV Series")
	}
	if r.Studio != "CBC" {
		t.Errorf("Studio: got %q, want %q", r.Studio, "CBC")
	}
}

func TestIsCreativeBCRelevant_featureFilm(t *testing.T) {
	rec := creativeBCRecord{Title: "Test", Type: "Feature Film", Studio: "Netflix"}
	if !isCreativeBCRelevant(rec) {
		t.Error("Feature Film should pass filter")
	}
}

func TestIsCreativeBCRelevant_tvSeries(t *testing.T) {
	rec := creativeBCRecord{Title: "Test", Type: "TV Series", Studio: "CBC"}
	if !isCreativeBCRelevant(rec) {
		t.Error("TV Series should pass filter")
	}
}

func TestIsCreativeBCRelevant_animation(t *testing.T) {
	rec := creativeBCRecord{Title: "Test", Type: "Animation Series", Studio: "DHX"}
	if isCreativeBCRelevant(rec) {
		t.Error("Animation Series should be filtered out")
	}
}

func TestIsCreativeBCRelevant_documentary(t *testing.T) {
	rec := creativeBCRecord{Title: "Test", Type: "Documentary Series", Studio: "NatGeo"}
	if isCreativeBCRelevant(rec) {
		t.Error("Documentary Series should be filtered out")
	}
}

func TestToCreativeBCRawProject_fields(t *testing.T) {
	rec := creativeBCRecord{
		Title:  "The Lost Highway",
		Type:   "Feature Film",
		Studio: "Netflix",
		Status: "Principal Photography",
	}

	p := toCreativeBCRawProject(rec)

	if p.Source != "creativebc" {
		t.Errorf("Source: got %q, want %q", p.Source, "creativebc")
	}
	if p.ExternalID != "the-lost-highway" {
		t.Errorf("ExternalID: got %q, want %q", p.ExternalID, "the-lost-highway")
	}
	if p.Title != "The Lost Highway" {
		t.Errorf("Title: got %q, want %q", p.Title, "The Lost Highway")
	}
	if p.Value != 0 {
		t.Errorf("Value: got %d, want 0 (unknown)", p.Value)
	}
	applicant, _ := p.RawData["applicant"].(string)
	if applicant != "Netflix" {
		t.Errorf("RawData[applicant]: got %q, want %q", applicant, "Netflix")
	}
	prodType, _ := p.RawData["production_type"].(string)
	if prodType != "Feature Film" {
		t.Errorf("RawData[production_type]: got %q, want %q", prodType, "Feature Film")
	}
}

func TestHashCreativeBCProduction_deterministic(t *testing.T) {
	h1 := hashCreativeBCProduction("The Lost Highway", "Feature Film")
	h2 := hashCreativeBCProduction("The Lost Highway", "Feature Film")
	if h1 != h2 {
		t.Error("hash is not deterministic")
	}
}

func TestHashCreativeBCProduction_caseInsensitive(t *testing.T) {
	h1 := hashCreativeBCProduction("The Lost Highway", "Feature Film")
	h2 := hashCreativeBCProduction("the lost highway", "feature film")
	if h1 != h2 {
		t.Error("hash should be case-insensitive for stable dedup")
	}
}

func TestHashCreativeBCProduction_differentProductions(t *testing.T) {
	h1 := hashCreativeBCProduction("The Lost Highway", "Feature Film")
	h2 := hashCreativeBCProduction("Vancouver Chronicles", "TV Series")
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

// sampleCreativeBCLinesNoHeader tests the fallback type-anchor parser.
var sampleCreativeBCLinesNoHeader = []string{
	`The Lost Highway    Feature Film    Netflix    Principal Photography`,
	`Vancouver Chronicles    TV Series    CBC    Pre-Production`,
	`Burnaby Blocks    Animation Series    DHX`,
}

func TestParseCreativeBCByTypeAnchor(t *testing.T) {
	records := parseCreativeBCByTypeAnchor(sampleCreativeBCLinesNoHeader)

	// Should parse 3 records (anchor parser finds any known type)
	if len(records) != 3 {
		t.Fatalf("expected 3 records from anchor parser, got %d", len(records))
	}

	// First record title check
	if records[0].Title != "The Lost Highway" {
		t.Errorf("first record Title: got %q, want %q", records[0].Title, "The Lost Highway")
	}
	if records[0].Type != "Feature Film" {
		t.Errorf("first record Type: got %q, want %q", records[0].Type, "Feature Film")
	}
}
