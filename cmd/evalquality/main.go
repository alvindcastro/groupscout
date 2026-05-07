package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/alvindcastro/groupscout/internal/evalops"
)

func main() {
	casePath := flag.String("cases", "data/evals/groupscout", "JSONL case file or directory")
	outputDir := flag.String("out", "build/evals", "directory for JSON, Markdown, and JUnit reports")
	flag.Parse()

	artifacts, report, err := evalops.RunQuality(context.Background(), evalops.QualityOptions{
		CasePaths: []string{*casePath},
		OutputDir: *outputDir,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval quality error:", err)
		os.Exit(2)
	}
	fmt.Printf("eval quality complete: total=%d passed=%d critical=%d warnings=%d release_blocking=%d\n",
		report.Summary.Total,
		report.Summary.Passed,
		report.Summary.CriticalFailures,
		report.Summary.Warnings,
		report.Summary.ReleaseBlockingFailures,
	)
	fmt.Printf("reports: %s %s %s\n", artifacts.JSONPath, artifacts.MarkdownPath, artifacts.JUnitPath)
}
