package permits

import (
	"os"
	"testing"
	"time"
)

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

	p := toRawProject(rec, []byte("test pdf content"))

	if p.Source != "richmond_permits" {
		t.Errorf("Source = %q, want %q", p.Source, "richmond_permits")
	}
	if p.ExternalID != rec.FolderNumber {
		t.Errorf("ExternalID = %q, want %q", p.ExternalID, rec.FolderNumber)
	}
	if p.Location != "8640 Alexandra Road, Richmond BC" {
		t.Errorf("Location = %q, want %q", p.Location, "8640 Alexandra Road, Richmond BC")
	}
	if p.Value != rec.ValueCAD {
		t.Errorf("Value = %d, want %d", p.Value, rec.ValueCAD)
	}
	if p.IssuedAt != date {
		t.Errorf("IssuedAt = %v, want %v", p.IssuedAt, date)
	}
	if string(p.RawData) != "test pdf content" {
		t.Errorf("RawData = %q, want %q", string(p.RawData), "test pdf content")
	}
	if p.RawType != "application/pdf" {
		t.Errorf("RawType = %q, want %q", p.RawType, "application/pdf")
	}
}

// ── parseBBoxLayout ──────────────────────────────────────────────────────────

// testdata/richmond_oct4-10_2026.bbox.html is `pdftotext -bbox-layout` output for
// the real Oct 4–10, 2026 weekly report (buildingreportoct4-10202679898.pdf).
func TestParseBBoxLayout(t *testing.T) {
	data, err := os.ReadFile("testdata/richmond_oct4-10_2026.bbox.html")
	if err != nil {
		t.Fatal(err)
	}
	records, err := parseBBoxLayout(data)
	if err != nil {
		t.Fatalf("parseBBoxLayout: %v", err)
	}

	// The report's Grand Totals row counts 22 permits.
	if len(records) != 22 {
		t.Fatalf("got %d records, want 22", len(records))
	}

	byFolder := map[string]permitRecord{}
	for _, r := range records {
		byFolder[r.FolderNumber] = r
	}

	t.Run("warehouse permit on the last page", func(t *testing.T) {
		r, ok := byFolder["26 014001 000 00 BB"]
		if !ok {
			t.Fatal("warehouse permit 26 014001 000 00 BB not parsed")
		}
		want := permitRecord{
			SubType:      "Warehouse",
			FolderNumber: "26 014001 000 00 BB",
			WorkProposed: "Alteration",
			Status:       "Issued",
			IssueDate:    time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			ValueCAD:     2_000_000,
			Address:      "2471 Viking Way",
			Applicant:    "Jack Zhang (604)761-9246",
			Contractor:   "Chandos Construction (778)372-6940",
		}
		if r != want {
			t.Errorf("got  %+v\nwant %+v", r, want)
		}
		if !isRelevant(r, minPermitValueCAD) {
			t.Error("$2M warehouse alteration should pass isRelevant")
		}
	})

	t.Run("hotel section with compressed header", func(t *testing.T) {
		r := byFolder["26 024110 000 00 B7"]
		if r.SubType != "Hotel" || r.WorkProposed != "Special Inspection" || r.Status != "Issued" {
			t.Errorf("SubType/Work/Status = %q/%q/%q, want Hotel/Special Inspection/Issued", r.SubType, r.WorkProposed, r.Status)
		}
		if r.Address != "2888 Jow Street - Hotel 1- SPECIAL INSPECTION" {
			t.Errorf("Address = %q", r.Address)
		}
		if r.Contractor != "Scott Construction Management Ltd (604) 874-8228" {
			t.Errorf("Contractor = %q", r.Contractor)
		}
	})

	t.Run("multi-permit section keeps each value", func(t *testing.T) {
		// Restaurant section: subtotal row $200,000.00 must not overwrite permit values.
		if v := byFolder["22 008943 000 01 B2"].ValueCAD; v != 0 {
			t.Errorf("22 008943 ValueCAD = %d, want 0", v)
		}
		if v := byFolder["26 019423 000 00 B2"].ValueCAD; v != 100_000 {
			t.Errorf("26 019423 ValueCAD = %d, want 100000", v)
		}
	})

	t.Run("non-issued status", func(t *testing.T) {
		r := byFolder["26 023979 000 00 BZ"]
		if r.WorkProposed != "New" || r.Status != "Pickup - Deposit" {
			t.Errorf("Work/Status = %q/%q, want New/Pickup - Deposit", r.WorkProposed, r.Status)
		}
	})

	t.Run("only the warehouse permit is relevant", func(t *testing.T) {
		var relevant []string
		for _, r := range records {
			if isRelevant(r, minPermitValueCAD) {
				relevant = append(relevant, r.FolderNumber)
			}
		}
		if len(relevant) != 1 || relevant[0] != "26 014001 000 00 BB" {
			t.Errorf("relevant permits = %v, want [26 014001 000 00 BB]", relevant)
		}
	})
}
