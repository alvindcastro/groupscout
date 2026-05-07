package evalops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunQualityWritesJSONMarkdownAndJUnitArtifacts(t *testing.T) {
	caseDir := t.TempDir()
	casePath := filepath.Join(caseDir, "lead.jsonl")
	writeEvalFixture(t, casePath, `{"id":"lead-quality-001","case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/richmond/lead-quality-001.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"Airport Logistics Centre Phase 2","text":"Building permit summary: new logistics warehouse near airport. Declared value CAD 8400000. Estimated construction schedule 14 months. Site staging calls for structural steel crews.","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":8400000,"metadata":{"duration_months":14}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":8,"max":10},"enrichment":{"project_type":"industrial construction","estimated_room_nights":{"min":1200,"max":4200,"unknown_allowed":false},"project_duration_months":{"min":10,"max":18,"unknown_allowed":false},"lodging_need":"high","confidence":"medium"},"alert":null,"evidence":[{"claim":"large commercial construction","must_support_with":"CAD 8400000 warehouse and 14 month schedule"}],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":["drops the lead"]}}`)
	outDir := t.TempDir()

	artifacts, report, err := RunQuality(context.Background(), QualityOptions{
		CasePaths: []string{caseDir},
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("RunQuality() error = %v", err)
	}
	if report.Summary.Total == 0 || report.Summary.CriticalFailures != 0 {
		t.Fatalf("report summary = %+v, want passing scored report", report.Summary)
	}
	for _, path := range []string{artifacts.JSONPath, artifacts.MarkdownPath, artifacts.JUnitPath} {
		if path == "" {
			t.Fatalf("artifact path is empty: %+v", artifacts)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("artifact %s not written: %v", path, err)
		}
	}
	markdown, err := os.ReadFile(artifacts.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown artifact: %v", err)
	}
	if !strings.Contains(string(markdown), "GroupScout EvalOps Report") {
		t.Fatalf("markdown artifact missing report heading:\n%s", markdown)
	}
}

func TestRunQualityGoldenFixturesPassWithoutCriticalFailures(t *testing.T) {
	_, report, err := RunQuality(context.Background(), QualityOptions{
		CasePaths: []string{filepath.Join("..", "..", "data", "evals", "groupscout")},
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunQuality() error = %v", err)
	}
	if report.Summary.CriticalFailures != 0 || report.Summary.ReleaseBlockingFailures != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("golden fixture report summary = %+v, want all passing", report.Summary)
	}
}
