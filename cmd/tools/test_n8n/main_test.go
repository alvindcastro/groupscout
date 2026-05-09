package main

import (
	"encoding/json"
	"io"
	"testing"
)

func TestNewLeadRequest(t *testing.T) {
	req, err := newLeadRequest("http://localhost:8080/n8n/webhook", "secret-token")
	if err != nil {
		t.Fatalf("newLeadRequest: %v", err)
	}

	if req.Method != "POST" {
		t.Fatalf("method = %q, want POST", req.Method)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer secret-token", got)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var lead map[string]any
	if err := json.Unmarshal(body, &lead); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if lead["source"] != "n8n-test" {
		t.Fatalf("source = %v, want n8n-test", lead["source"])
	}
	if lead["priority_score"] != float64(9) {
		t.Fatalf("priority_score = %v, want 9", lead["priority_score"])
	}
}
