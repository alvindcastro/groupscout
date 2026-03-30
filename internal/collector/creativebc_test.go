package collector

import (
	"testing"
)

// sampleCreativeBCHTML is a minimal HTML fixture that mirrors the structure expected
// from the Creative BC Visualforce page.
var sampleCreativeBCHTML = []byte(`<!DOCTYPE html>
<html>
<body>
<table>
  <thead>
    <tr>
      <th>Production Title</th>
      <th>Production Type</th>
      <th>Studio/Distributor</th>
      <th>Status</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>The Lost Highway</td>
      <td>Feature Film</td>
      <td>Netflix</td>
      <td>Principal Photography</td>
    </tr>
    <tr>
      <td>Vancouver Chronicles</td>
      <td>TV Series</td>
      <td>CBC</td>
      <td>Pre-Production</td>
    </tr>
    <tr>
      <td>Mountain Light</td>
      <td>Feature Film</td>
      <td>A24</td>
      <td>Pre-Production</td>
    </tr>
    <tr>
      <td>Burnaby Blocks</td>
      <td>Animation Series</td>
      <td>DHX Media</td>
      <td>Production</td>
    </tr>
    <tr>
      <td>Northern Exposure</td>
      <td>Documentary Series</td>
      <td>National Geographic</td>
      <td>Post Production</td>
    </tr>
  </tbody>
</table>
</body>
</html>`)

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

func TestParseCreativeBCHTML_secondRecord(t *testing.T) {
	records, err := parseCreativeBCHTML(sampleCreativeBCHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := records[1]

	if r.Title != "Vancouver Chronicles" {
		t.Errorf("Title: got %q, want %q", r.Title, "Vancouver Chronicles")
	}
	if r.Type != "TV Series" {
		t.Errorf("Type: got %q, want %q", r.Type, "TV Series")
	}
}

func TestParseCreativeBCHTML_noTable(t *testing.T) {
	_, err := parseCreativeBCHTML([]byte(`<html><body><p>No table here</p></body></html>`))
	if err == nil {
		t.Error("expected error when no production table found")
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
	if p.Value != 0 {
		t.Errorf("Value: got %d, want 0", p.Value)
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
		t.Error("hash should be case-insensitive")
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
