package permits

import (
	"testing"
	"time"
)

// sampleDeltaLines is a realistic excerpt from pdftotext -layout output on a Delta permit PDF.
// Columns preserved with spaces as pdftotext emits them.
var sampleDeltaLines = []string{
	`Mar 3, 2026  BP022784 MCRAE, ALICIA                                                              0.00          515,000.00`,
	`             Type INDUSTRIAL - TILBURY`,
	`             Purpose Interior Tenant Improvement`,
	`             Folio:                     Civic Address:`,
	`             Folio: 344-932-31-0        Civic Address:          6705 DENNETT PL`,
	``,
	`Mar 10, 2026  BP022801 DELTA DEVELOPMENTS INC                                                   850.00      1,200,000.00`,
	`             Type COMMERCIAL - LADNER`,
	`             Purpose New Commercial Building`,
	`             Folio: 123-456-78-9        Civic Address:          4500 CLARENCE TAYLOR CRES`,
	``,
	`Mar 10, 2026  BP022802 SMITH, JOHN                                                                 0.00         35,000.00`,
	`             Type RESIDENTIAL - NORTH DELTA`,
	`             Purpose Single Family Dwelling`,
	`             Folio: 999-111-22-3        Civic Address:          123 MAPLE ST`,
}

func TestParseDeltaPermitLines_count(t *testing.T) {
	records := parseDeltaPermitLines(sampleDeltaLines)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestParseDeltaPermitLines_firstRecord(t *testing.T) {
	records := parseDeltaPermitLines(sampleDeltaLines)
	r := records[0]

	if r.PermitNumber != "BP022784" {
		t.Errorf("PermitNumber: got %q, want %q", r.PermitNumber, "BP022784")
	}
	if r.ValueCAD != 515_000 {
		t.Errorf("ValueCAD: got %d, want 515000", r.ValueCAD)
	}
	if r.TypeRaw != "INDUSTRIAL - TILBURY" {
		t.Errorf("TypeRaw: got %q, want %q", r.TypeRaw, "INDUSTRIAL - TILBURY")
	}
	if r.TypePrefix != "industrial" {
		t.Errorf("TypePrefix: got %q, want %q", r.TypePrefix, "industrial")
	}
	if r.Purpose != "Interior Tenant Improvement" {
		t.Errorf("Purpose: got %q, want %q", r.Purpose, "Interior Tenant Improvement")
	}
	if r.CivicAddress != "6705 DENNETT PL" {
		t.Errorf("CivicAddress: got %q, want %q", r.CivicAddress, "6705 DENNETT PL")
	}
	want := time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	if !r.IssueDate.Equal(want) {
		t.Errorf("IssueDate: got %v, want %v", r.IssueDate, want)
	}
}

func TestParseDeltaPermitLines_secondRecord(t *testing.T) {
	records := parseDeltaPermitLines(sampleDeltaLines)
	r := records[1]

	if r.PermitNumber != "BP022801" {
		t.Errorf("PermitNumber: got %q, want %q", r.PermitNumber, "BP022801")
	}
	if r.ValueCAD != 1_200_000 {
		t.Errorf("ValueCAD: got %d, want 1200000", r.ValueCAD)
	}
	if r.TypePrefix != "commercial" {
		t.Errorf("TypePrefix: got %q, want %q", r.TypePrefix, "commercial")
	}
}

func TestIsDeltaRelevant_passes(t *testing.T) {
	rec := deltaRecord{TypePrefix: "industrial", ValueCAD: 600_000}
	if !isDeltaRelevant(rec, 500_000) {
		t.Error("expected industrial permit at $600k to pass filter")
	}
}

func TestIsDeltaRelevant_belowThreshold(t *testing.T) {
	rec := deltaRecord{TypePrefix: "industrial", ValueCAD: 400_000}
	if isDeltaRelevant(rec, 500_000) {
		t.Error("expected permit below threshold to be filtered out")
	}
}

func TestIsDeltaRelevant_residential(t *testing.T) {
	rec := deltaRecord{TypePrefix: "residential", ValueCAD: 2_000_000}
	if isDeltaRelevant(rec, 500_000) {
		t.Error("expected residential permit to be filtered out regardless of value")
	}
}

func TestParseDeltaDecimal(t *testing.T) {
	cases := []struct {
		input string
		want  int64
	}{
		{"515,000.00", 515_000},
		{"1,200,000.00", 1_200_000},
		{"0.00", 0},
		{"850.00", 850},
	}
	for _, c := range cases {
		got := parseDeltaDecimal(c.input)
		if got != c.want {
			t.Errorf("parseDeltaDecimal(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestHashDeltaPermit_deterministic(t *testing.T) {
	d := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	h1 := hashDeltaPermit("BP022784", "6705 DENNETT PL", d)
	h2 := hashDeltaPermit("BP022784", "6705 DENNETT PL", d)
	if h1 != h2 {
		t.Error("hash is not deterministic")
	}
}

func TestHashDeltaPermit_differentPermits(t *testing.T) {
	d := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	h1 := hashDeltaPermit("BP022784", "6705 DENNETT PL", d)
	h2 := hashDeltaPermit("BP022801", "4500 CLARENCE TAYLOR CRES", d)
	if h1 == h2 {
		t.Error("different permits should produce different hashes")
	}
}

func TestToDeltaRawProject_fields(t *testing.T) {
	rec := deltaRecord{
		PermitNumber: "BP022784",
		Builder:      "MCRAE, ALICIA",
		IssueDate:    time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
		ValueCAD:     515_000,
		TypeRaw:      "INDUSTRIAL - TILBURY",
		TypePrefix:   "industrial",
		Purpose:      "Interior Tenant Improvement",
		CivicAddress: "6705 DENNETT PL",
	}

	rawData := []byte("fake delta pdf")
	p := toDeltaRawProject(rec, rawData)

	if p.Source != "delta_permits" {
		t.Errorf("Source: got %q, want %q", p.Source, "delta_permits")
	}
	if p.ExternalID != "BP022784" {
		t.Errorf("ExternalID: got %q, want %q", p.ExternalID, "BP022784")
	}
	if p.Value != 515_000 {
		t.Errorf("Value: got %d, want 515000", p.Value)
	}
	if string(p.RawData) != "fake delta pdf" {
		t.Errorf("RawData mismatch")
	}
	if p.RawType != "application/pdf" {
		t.Errorf("RawType: got %q, want %q", p.RawType, "application/pdf")
	}
}
