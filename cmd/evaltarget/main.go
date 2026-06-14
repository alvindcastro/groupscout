// Command evaltarget starts the local eval target HTTP server for Promptfoo.
// It loads golden fixture cases from data/evals/groupscout/ and serves
// POST /eval/run requests, running deterministic scorers against each case.
//
// Usage:
//
//	go run cmd/evaltarget/main.go
//	# or: make eval-target
//
// The server listens on :18080 by default. Override with EVAL_TARGET_PORT.
// No live APIs, credentials, or external services are used.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alvindcastro/groupscout/internal/evalops"
)

func main() {
	port := os.Getenv("EVAL_TARGET_PORT")
	if port == "" {
		port = "18080"
	}

	// Load fixture cases from the canonical fixture directory.
	// This path is relative to the repo root when run with `go run` or `make eval-target`.
	fixtureDir := "data/evals/groupscout"
	if dir := os.Getenv("EVAL_FIXTURE_DIR"); dir != "" {
		fixtureDir = dir
	}

	absDir, err := filepath.Abs(fixtureDir)
	if err != nil {
		log.Fatalf("evaltarget: resolve fixture dir %q: %v", fixtureDir, err)
	}

	cases, err := evalops.LoadCasesFromDir(absDir)
	if err != nil {
		log.Fatalf("evaltarget: load cases from %q: %v", absDir, err)
	}

	log.Printf("evaltarget: loaded %d fixture cases from %s", len(cases), absDir)

	handler := evalops.NewEvalTargetHandler(cases, 10*time.Second)
	mux := http.NewServeMux()
	mux.Handle("/eval/run", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","cases":%d}`, len(cases))
	})

	addr := ":" + port
	log.Printf("evaltarget: listening on %s (no live APIs)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("evaltarget: server error: %v", err)
	}
}
