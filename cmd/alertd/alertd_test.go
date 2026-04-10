package main

import (
	"testing"
	"time"
)

func TestPollInterval_QuietMode(t *testing.T) {
	interval := getPollInterval(false)
	expected := 10 * time.Minute
	if interval != expected {
		t.Errorf("expected %v, got %v", expected, interval)
	}
}

func TestPollInterval_ActiveAlertMode(t *testing.T) {
	interval := getPollInterval(true)
	expected := 90 * time.Second
	if interval != expected {
		t.Errorf("expected %v, got %v", expected, interval)
	}
}

func TestBackoff_Max5Min(t *testing.T) {
	// Test exponential backoff: 2^n * base, capped at 5 min
	base := 10 * time.Second
	max := 5 * time.Minute

	// After 1 error
	b := computeBackoff(1, base, max)
	if b != 10*time.Second {
		t.Errorf("expected 10s, got %v", b)
	}

	// After 5 errors: 10 * 2^4 = 160s
	b = computeBackoff(5, base, max)
	if b != 160*time.Second {
		t.Errorf("expected 160s, got %v", b)
	}

	// After 6 errors: 10 * 2^5 = 320s -> capped at 300s
	b = computeBackoff(6, base, max)
	if b != 300*time.Second {
		t.Errorf("expected 300s, got %v", b)
	}
}
