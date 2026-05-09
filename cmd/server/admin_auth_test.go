package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOrCreateSetupTokenPersistsGeneratedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-setup-token")

	first, err := readOrCreateSetupToken(path)
	if err != nil {
		t.Fatalf("readOrCreateSetupToken first call: %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("generated token length = %d, want 64 hex chars", len(first))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token file permissions = %v, want 0600", info.Mode().Perm())
	}

	second, err := readOrCreateSetupToken(path)
	if err != nil {
		t.Fatalf("readOrCreateSetupToken second call: %v", err)
	}
	if second != first {
		t.Fatalf("second token = %q, want persisted first token %q", second, first)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if strings.TrimSpace(string(data)) != first {
		t.Fatalf("token file contents did not match generated token")
	}
}
