package collector

import (
	"testing"
	"time"
)

// sampleLines is a representative slice of text lines as they come out of parsePDF().
// Derived from the real Richmond weekly building report (Mar 15–21, 2026).
//
// Matches the real pdftotext output structure: permit records (left column) come first,
// then APPLICANT/CONTRACTOR blocks (right column) appear after all permits on the page.
// Each APPLICANT block corresponds to one SUB TYPE section in the same order.
var sampleLines = []string{
	// Page chrome — skipped
	"Building Permit Issuance Report",
	"City of Richmond",
	"Filters",
	"ISSUED FROM Equal to 15/03/2026 And ISSUED TO Equal to 21/03/2026",

	// Section 0: Hotel
	"SUB TYPE: Hotel",
	"FOLDER NUMBER WORK PROPOSED STATUS ISSUE DATE CONSTR. VALUE", // column header — skipped
	"25 036523 000 00 B7",
	"Alteration",
	"Issued",
	"2026/03/16",
	"1",             // permit count row — skipped
	"CONSTR. VALUE", // column header repeat — skipped
	"$300,000.00",
	"FOLDER NAME 8640 Alexandra Road",
	"$300,000.00", // section subtotal — second value, skipped

	// Section 1: Warehouse
	"SUB TYPE: Warehouse",
	"24 008734 000 01 B7",
	"New",
	"Issued",
	"2026/03/18",
	"1",
	"$1,200,000.00",
	"FOLDER NAME 12500 Vulcan Way",
	"$1,200,000.00",

	// Section 2: One Family Dwelling (residential — filtered by isRelevant)
	"SUB TYPE: One Family Dwelling",
	"25 011111 000 00 B7",
	"New",
	"Issued",
	"2026/03/19",
	"1",
	"$850,000.00",
	"FOLDER NAME 9800 Maple Street",
	"$850,000.00",

	"SUB TOTAL",   // should be skipped
	"GRAND TOTAL", // should be skipped

	// Right column — APPLICANT/CONTRACTOR blocks in section order (0, 1, 2)
	// Block 0 → Hotel (sectionIdx = 0)
	"APPLICANT",
	"Studio Senbel Architecture and Design Inc (Sharif Senbel) (604)605-6995",
	"CONTRACTOR Safara Cladding Inc (416)875-1770",

	// Block 1 → Warehouse (sectionIdx = 1)
	"APPLICANT ABC Developments Ltd (604)555-0100",
	"CONTRACTOR BuildRight Contracting (604)555-0199",

	// Block 2 → One Family Dwelling (sectionIdx = 2)
	"APPLICANT John Smith",
	"CONTRACTOR Smith Build Co",
}

// ── parseDollarAmount ────────────────────────────────────────────────────────

func TestParseDollarAmount(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"$300,000.00", 300_000},
		{"$1,200,000.00", 1_200_000},
		{"$57,290,092.00", 57_290_092},
		{"$664,886.30", 664_886},
		{"$0.00", 0},
		{"$500,000", 500_000},
		{"", 0},
		{"N/A", 0},
	}

	for _, tt := range tests {
		got := parseDollarAmount(tt.input)
		if got != tt.want {
			t.Errorf("parseDollarAmount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// ── isRelevant ───────────────────────────────────────────────────────────────

func TestIsRelevant(t *testing.T) {
	tests := []struct {
		name    string
		subType string
		value   int64
		want    bool
	}{
		// Pass — commercial + above threshold
		{"hotel above threshold", "Hotel", 600_000, true},
		{"warehouse above threshold", "Warehouse", 1_200_000, true},
		{"office above threshold", "Office", 500_001, true},
		{"restaurant above threshold", "Restaurant", 750_000, true},
		{"apartment above threshold", "Apartment", 2_000_000, true},

		// Fail — commercial but below threshold
		{"hotel below threshold", "Hotel", 499_999, false},
		{"warehouse below threshold", "Warehouse", 100_000, false},
		{"office at threshold", "Office", 500_000, false}, // must be > not >=

		// Fail — residential (any value)
		{"one family dwelling", "One Family Dwelling", 900_000, false},
		{"townhouse", "Townhouse", 1_500_000, false},
		{"single family suite", "Single Family/Suite", 800_000, false},

		// Pass — case insensitive
		{"hotel lowercase", "hotel", 600_000, true},
		{"WAREHOUSE uppercase", "WAREHOUSE", 600_000, true},
	}

	for _, tt := range tests {
		rec := permitRecord{SubType: tt.subType, ValueCAD: tt.value}
		got := isRelevant(rec, 500_000)
		if got != tt.want {
			t.Errorf("[%s] isRelevant({SubType:%q, ValueCAD:%d}) = %v, want %v",
				tt.name, tt.subType, tt.value, got, tt.want)
		}
	}
}

// ── hashPermit ───────────────────────────────────────────────────────────────

func TestHashPermit(t *testing.T) {
	date := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)

	t.Run("deterministic", func(t *testing.T) {
		h1 := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date)
		h2 := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date)
		if h1 != h2 {
			t.Errorf("same inputs produced different hashes: %q vs %q", h1, h2)
		}
	})

	t.Run("different folder numbers produce different hashes", func(t *testing.T) {
		h1 := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date)
		h2 := hashPermit("24 008734 000 01 B7", "8640 Alexandra Road", date)
		if h1 == h2 {
			t.Error("different folder numbers produced the same hash")
		}
	})

	t.Run("different dates produce different hashes", func(t *testing.T) {
		date2 := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
		h1 := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date)
		h2 := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date2)
		if h1 == h2 {
			t.Error("different dates produced the same hash")
		}
	})

	t.Run("non-empty", func(t *testing.T) {
		h := hashPermit("25 036523 000 00 B7", "8640 Alexandra Road", date)
		if h == "" {
			t.Error("hash should not be empty")
		}
	})
}

// ── toRawProject ─────────────────────────────────────────────────────────────

func TestToRawProject(t *testing.T) {
	date := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	rec := permitRecord{
		SubType:      "Hotel",
		FolderNumber: "25 036523 000 00 B7",
		WorkProposed: "Alteration",
		Status:       "Issued",
		IssueDate:    date,
		ValueCAD:     300_000,
		Address:      "8640 Alexandra Road",
		Applicant:    "Studio Senbel Architecture Inc",
		Contractor:   "Safara Cladding Inc",
	}

	p := toRawProject(rec)

	if p.Source != "richmond_permits" {
		t.Errorf("Source = %q, want %q", p.Source, "richmond_permits")
	}
	if p.ExternalID != rec.FolderNumber {
		t.Errorf("ExternalID = %q, want %q", p.ExternalID, rec.FolderNumber)
	}
	if p.Location != rec.Address {
		t.Errorf("Location = %q, want %q", p.Location, rec.Address)
	}
	if p.Value != rec.ValueCAD {
		t.Errorf("Value = %d, want %d", p.Value, rec.ValueCAD)
	}
	if p.IssuedAt != date {
		t.Errorf("IssuedAt = %v, want %v", p.IssuedAt, date)
	}
	// RawData must preserve all original fields
	for _, key := range []string{"folder_number", "sub_type", "work_proposed", "status", "issue_date", "value_cad", "address", "applicant", "contractor"} {
		if _, ok := p.RawData[key]; !ok {
			t.Errorf("RawData missing key %q", key)
		}
	}
}

// ── parsePermitLines ─────────────────────────────────────────────────────────

func TestParsePermitLines(t *testing.T) {
	records := parsePermitLines(sampleLines)

	// sampleLines has 3 permits: Hotel, Warehouse, One Family Dwelling
	if len(records) != 3 {
		t.Fatalf("expected 3 permit records, got %d", len(records))
	}

	hotel := records[0]
	warehouse := records[1]
	residential := records[2]

	t.Run("hotel sub-type", func(t *testing.T) {
		if hotel.SubType != "Hotel" {
			t.Errorf("SubType = %q, want %q", hotel.SubType, "Hotel")
		}
		if hotel.FolderNumber != "25 036523 000 00 B7" {
			t.Errorf("FolderNumber = %q, want %q", hotel.FolderNumber, "25 036523 000 00 B7")
		}
		if hotel.ValueCAD != 300_000 {
			t.Errorf("ValueCAD = %d, want 300000", hotel.ValueCAD)
		}
		if hotel.Address != "8640 Alexandra Road" {
			t.Errorf("Address = %q, want %q", hotel.Address, "8640 Alexandra Road")
		}
		if hotel.IssueDate.IsZero() {
			t.Error("IssueDate should not be zero")
		}
	})

	t.Run("hotel applicant and contractor from right-column block", func(t *testing.T) {
		wantApplicant := "Studio Senbel Architecture and Design Inc (Sharif Senbel) (604)605-6995"
		wantContractor := "Safara Cladding Inc (416)875-1770"
		if hotel.Applicant != wantApplicant {
			t.Errorf("Applicant = %q, want %q", hotel.Applicant, wantApplicant)
		}
		if hotel.Contractor != wantContractor {
			t.Errorf("Contractor = %q, want %q", hotel.Contractor, wantContractor)
		}
	})

	t.Run("warehouse sub-type", func(t *testing.T) {
		if warehouse.SubType != "Warehouse" {
			t.Errorf("SubType = %q, want %q", warehouse.SubType, "Warehouse")
		}
		if warehouse.ValueCAD != 1_200_000 {
			t.Errorf("ValueCAD = %d, want 1200000", warehouse.ValueCAD)
		}
	})

	t.Run("warehouse applicant and contractor", func(t *testing.T) {
		if warehouse.Applicant != "ABC Developments Ltd (604)555-0100" {
			t.Errorf("Applicant = %q, want %q", warehouse.Applicant, "ABC Developments Ltd (604)555-0100")
		}
		if warehouse.Contractor != "BuildRight Contracting (604)555-0199" {
			t.Errorf("Contractor = %q, want %q", warehouse.Contractor, "BuildRight Contracting (604)555-0199")
		}
	})

	t.Run("residential sub-type parsed but filtered by isRelevant", func(t *testing.T) {
		// parsePermitLines returns all records — filtering is isRelevant's job
		if residential.SubType != "One Family Dwelling" {
			t.Errorf("SubType = %q, want %q", residential.SubType, "One Family Dwelling")
		}
		if isRelevant(residential, 500_000) {
			t.Error("residential permit should not pass isRelevant filter")
		}
	})

	t.Run("page chrome and headers are skipped", func(t *testing.T) {
		for _, rec := range records {
			if rec.SubType == "GRAND TOTAL" || rec.SubType == "SUB TOTAL" {
				t.Errorf("totals should not appear as sub-types, got %q", rec.SubType)
			}
			if rec.FolderNumber == "FOLDER NUMBER WORK PROPOSED STATUS ISSUE DATE CONSTR. VALUE FOLDER NAME APPLICANT CONTRACTOR" {
				t.Error("column header row should be skipped")
			}
		}
	})
}
