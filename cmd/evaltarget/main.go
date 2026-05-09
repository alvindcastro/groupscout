package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/evalops"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "HTTP listen address")
	casePath := flag.String("cases", "data/evals/groupscout", "JSONL case file or directory")
	timeout := flag.Duration("timeout", 5*time.Second, "per-request eval timeout")
	flag.Parse()

	cases, err := evalops.LoadCases(*casePath)
	if err != nil {
		log.Fatalf("load eval cases: %v", err)
	}
	target := evalops.NewEvalTarget(cases, evalops.EvalTargetOptions{Timeout: *timeout})

	log.Printf("serving eval target on http://%s/eval/run with %d cases", *addr, len(cases))
	if err := http.ListenAndServe(*addr, target.Handler()); err != nil {
		log.Fatal(err)
	}
}
