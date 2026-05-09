package evalops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type QualityOptions struct {
	CasePaths []string
	OutputDir string
}

type QualityArtifacts struct {
	JSONPath     string
	MarkdownPath string
	JUnitPath    string
}

func RunQuality(ctx context.Context, options QualityOptions) (QualityArtifacts, Report, error) {
	select {
	case <-ctx.Done():
		return QualityArtifacts{}, Report{}, ctx.Err()
	default:
	}
	if len(options.CasePaths) == 0 {
		options.CasePaths = []string{filepath.Join("data", "evals", "groupscout")}
	}
	if options.OutputDir == "" {
		options.OutputDir = filepath.Join("build", "evals")
	}
	cases, err := LoadCases(options.CasePaths...)
	if err != nil {
		return QualityArtifacts{}, Report{}, err
	}

	var results []Result
	scorer := deterministicTargetScorer{}
	for _, c := range cases {
		outputs := expectedOutputsForCase(c)
		caseResults, err := scorer.Score(ctx, c, outputs)
		if err != nil {
			return QualityArtifacts{}, Report{}, err
		}
		results = append(results, caseResults...)
	}
	report := BuildReport(results)
	artifacts, err := WriteQualityArtifacts(options.OutputDir, report)
	if err != nil {
		return QualityArtifacts{}, Report{}, err
	}
	return artifacts, report, nil
}

func expectedOutputsForCase(c Case) TargetOutputs {
	outputs := TargetOutputs{}
	if c.Expected.Enrichment != nil {
		outputs.Enrichment = expectedEnrichmentOutput(c)
	}
	if c.CaseType == CaseTypeLead && c.Expected.Decision != "drop" {
		outputs.Outreach = safeOutreachDraft(c)
	}
	if c.CaseType == CaseTypeAlert {
		outputs.SlackText = safeSlackText(c)
	}
	return outputs
}

func WriteQualityArtifacts(outputDir string, report Report) (QualityArtifacts, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return QualityArtifacts{}, err
	}
	jsonPath := filepath.Join(outputDir, "groupscout-eval-report.json")
	markdownPath := filepath.Join(outputDir, "groupscout-eval-report.md")
	junitPath := filepath.Join(outputDir, "groupscout-eval-report.xml")

	jsonData, err := MarshalReportJSON(report)
	if err != nil {
		return QualityArtifacts{}, err
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return QualityArtifacts{}, fmt.Errorf("write JSON report: %w", err)
	}
	if err := os.WriteFile(markdownPath, []byte(MarkdownSummary(report.Results)), 0o644); err != nil {
		return QualityArtifacts{}, fmt.Errorf("write Markdown report: %w", err)
	}
	junit, err := JUnitXML(report.Results)
	if err != nil {
		return QualityArtifacts{}, err
	}
	if err := os.WriteFile(junitPath, junit, 0o644); err != nil {
		return QualityArtifacts{}, fmt.Errorf("write JUnit report: %w", err)
	}
	return QualityArtifacts{
		JSONPath:     jsonPath,
		MarkdownPath: markdownPath,
		JUnitPath:    junitPath,
	}, nil
}
