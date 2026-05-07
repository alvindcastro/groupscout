package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/alvindcastro/groupscout/internal/evalops"
)

func main() {
	reportPath := flag.String("report", "build/evals/groupscout-eval-report.json", "eval report JSON path")
	thresholdPath := flag.String("thresholds", "evals/promptfoo/thresholds.yaml", "gate threshold YAML path")
	flag.Parse()

	result, err := evalops.RunGate(context.Background(), evalops.GateOptions{
		ReportPath:    *reportPath,
		ThresholdPath: *thresholdPath,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval gate error:", err)
		os.Exit(2)
	}
	fmt.Println(result.Summary)
	os.Exit(result.ExitCode)
}
