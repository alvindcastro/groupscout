package evalops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ValidationError struct {
	File    string
	Line    int
	Message string
}

func (e ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s line %d: %s", filepath.Base(e.File), e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", filepath.Base(e.File), e.Message)
}

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

func LoadCases(paths ...string) ([]Case, error) {
	files, err := expandCasePaths(paths)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]ValidationError)
	var cases []Case
	var validationErrors ValidationErrors

	for _, path := range files {
		loaded, errs := loadCaseFile(path, seen)
		if len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
		}
		cases = append(cases, loaded...)
	}

	if len(validationErrors) > 0 {
		return nil, validationErrors
	}
	return cases, nil
}

func expandCasePaths(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(path, "*.jsonl"))
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		files = append(files, matches...)
	}
	return files, nil
}

func loadCaseFile(path string, seen map[string]ValidationError) ([]Case, ValidationErrors) {
	file, err := os.Open(path)
	if err != nil {
		return nil, ValidationErrors{{File: path, Message: err.Error()}}
	}
	defer file.Close()

	var cases []Case
	var errs ValidationErrors
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		rawLine := strings.TrimSpace(scanner.Text())
		if rawLine == "" {
			continue
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(rawLine), &fields); err != nil {
			errs = append(errs, ValidationError{File: path, Line: line, Message: "malformed JSON: " + err.Error()})
			continue
		}

		var c Case
		if err := json.Unmarshal([]byte(rawLine), &c); err != nil {
			errs = append(errs, ValidationError{File: path, Line: line, Message: "decode case: " + err.Error()})
			continue
		}

		lineErrs := validateCase(path, line, c, fields)
		if c.ID != "" {
			if previous, ok := seen[c.ID]; ok {
				lineErrs = append(lineErrs, ValidationError{
					File:    path,
					Line:    line,
					Message: fmt.Sprintf("duplicate id %s previously defined at %s line %d", c.ID, filepath.Base(previous.File), previous.Line),
				})
			} else {
				seen[c.ID] = ValidationError{File: path, Line: line}
			}
		}
		if len(lineErrs) > 0 {
			errs = append(errs, lineErrs...)
			continue
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, ValidationError{File: path, Line: line, Message: "read JSONL: " + err.Error()})
	}
	return cases, errs
}

func validateCase(path string, line int, c Case, fields map[string]json.RawMessage) ValidationErrors {
	var errs ValidationErrors
	add := func(message string) {
		errs = append(errs, ValidationError{File: path, Line: line, Message: message})
	}

	if _, ok := fields["id"]; !ok || c.ID == "" {
		add("missing id")
	}
	if _, ok := fields["case_type"]; !ok || c.CaseType == "" {
		add("missing case_type")
	} else if c.CaseType != CaseTypeLead && c.CaseType != CaseTypeAlert {
		add("unknown case_type " + string(c.CaseType))
	}
	if _, ok := fields["source"]; !ok {
		add("missing source")
	} else {
		if !validSourceSystem(c.Source.System) {
			add("unknown source system " + c.Source.System)
		}
		if !validSourceType(c.Source.Type) {
			add("unknown source type " + c.Source.Type)
		}
	}
	if _, ok := fields["raw"]; !ok {
		add("missing raw")
	}
	if _, ok := fields["expected"]; !ok {
		add("missing expected")
	} else {
		if c.Expected.EvalResult == "" || c.Expected.Decision == "" {
			add("missing expected outcome")
		}
		if !validDecision(c.CaseType, c.Expected.Decision) {
			add("unknown decision " + c.Expected.Decision)
		}
		if !validSeverity(c.Expected.SeverityOnMismatch) {
			add("unsupported severity_on_mismatch " + string(c.Expected.SeverityOnMismatch))
		}
	}
	if !validSeverity(c.RiskLevel) {
		add("unsupported risk_level " + string(c.RiskLevel))
	}
	return errs
}

func validSourceSystem(system string) bool {
	switch system {
	case "richmond_permits", "delta_permits", "creativebc", "vcc", "eventbrite", "bcbid", "announcements", "yvr_disruption":
		return true
	default:
		return false
	}
}

func validSourceType(sourceType string) bool {
	switch sourceType {
	case "permit_pdf", "html", "rss", "news_release", "multi_signal_snapshot":
		return true
	default:
		return false
	}
}

func validSeverity(severity Severity) bool {
	switch severity {
	case SeverityCritical, SeverityWarning, SeverityInfo:
		return true
	default:
		return false
	}
}

func validDecision(caseType CaseType, decision string) bool {
	switch caseType {
	case CaseTypeLead:
		return decision == "keep" || decision == "drop" || decision == "needs_review"
	case CaseTypeAlert:
		return decision == "ignore" || decision == "watch" || decision == "soft_alert" || decision == "hard_alert" || decision == "fail_closed"
	default:
		return false
	}
}
