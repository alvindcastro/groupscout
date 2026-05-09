package evalops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type GateOptions struct {
	ReportPath    string
	ThresholdPath string
}

type GateThresholds struct {
	MaxCriticalFailures        int  `yaml:"max_critical_failures" json:"max_critical_failures"`
	MaxReleaseBlockingFailures int  `yaml:"max_release_blocking_failures" json:"max_release_blocking_failures"`
	WarningsAsErrors           bool `yaml:"warnings_as_errors" json:"warnings_as_errors"`
}

type GateResult struct {
	Pass     bool
	ExitCode int
	Summary  string
	Report   Report
}

func RunGate(ctx context.Context, options GateOptions) (GateResult, error) {
	select {
	case <-ctx.Done():
		return GateResult{}, ctx.Err()
	default:
	}
	if options.ReportPath == "" {
		return GateResult{}, fmt.Errorf("report path is required")
	}
	if options.ThresholdPath == "" {
		return GateResult{}, fmt.Errorf("threshold path is required")
	}

	report, err := readReport(options.ReportPath)
	if err != nil {
		return GateResult{}, err
	}
	thresholds, err := readThresholds(options.ThresholdPath)
	if err != nil {
		return GateResult{}, err
	}
	return evaluateGate(report, thresholds), nil
}

func readReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read report: %w", err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("decode report JSON: %w", err)
	}
	if report.Summary.Total == 0 && len(report.Results) > 0 {
		report = BuildReport(report.Results)
	}
	return report, nil
}

func readThresholds(path string) (GateThresholds, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GateThresholds{}, fmt.Errorf("read thresholds: %w", err)
	}
	var thresholds GateThresholds
	if err := yaml.Unmarshal(data, &thresholds); err != nil {
		return GateThresholds{}, fmt.Errorf("decode thresholds YAML: %w", err)
	}
	return thresholds, nil
}

func evaluateGate(report Report, thresholds GateThresholds) GateResult {
	pass := true
	reason := "passed"
	if report.Summary.CriticalFailures > thresholds.MaxCriticalFailures {
		pass = false
		reason = "blocked"
	}
	if report.Summary.ReleaseBlockingFailures > thresholds.MaxReleaseBlockingFailures {
		pass = false
		reason = "blocked"
	}
	if thresholds.WarningsAsErrors && report.Summary.Warnings > 0 {
		pass = false
		reason = "blocked"
	}

	exitCode := 0
	if !pass {
		exitCode = 1
	}
	summary := fmt.Sprintf("eval gate %s: total=%d passed=%d critical=%d warnings=%d release_blocking=%d",
		reason,
		report.Summary.Total,
		report.Summary.Passed,
		report.Summary.CriticalFailures,
		report.Summary.Warnings,
		report.Summary.ReleaseBlockingFailures,
	)
	return GateResult{
		Pass:     pass,
		ExitCode: exitCode,
		Summary:  summary,
		Report:   report,
	}
}
