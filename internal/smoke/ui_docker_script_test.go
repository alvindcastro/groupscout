package smoke_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptPath(t *testing.T) string {
	t.Helper()
	// internal/smoke/ is two levels below repo root
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "scripts", "smoke-ui-docker-e2e.sh")
}

func readScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(scriptPath(t))
	if err != nil {
		t.Fatalf("smoke script not found at %s: %v", scriptPath(t), err)
	}
	return string(data)
}

func TestSmokeScriptExists(t *testing.T) {
	path := scriptPath(t)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("smoke script missing: %s: %v", path, err)
	}
}

func TestSmokeScriptHasRequiredChecks(t *testing.T) {
	script := readScript(t)
	required := []struct {
		label string
		token string
	}{
		{"strict mode", "set -euo pipefail"},
		{"backend health check", "/health"},
		{"UI Compose profile", "smoke-ui-e2e"},
		{"UI healthz endpoint", "/healthz"},
		{"UI root endpoint", "UI_PORT}/"},
		{"static asset check", "/assets/app.js"},
		{"proxied API smoke", "/api/system"},
		{"API_TOKEN secret scan", "API_TOKEN"},
		{"UI_SESSION_SECRET scan", "UI_SESSION_SECRET"},
		{"success message", "smoke-ui-docker-e2e: all checks passed"},
		{"cleanup trap", "cleanup"},
		{"groupscout-ui repo", "groupscout-ui"},
		{"configurable compose command", "GROUPSCOUT_COMPOSE"},
		{"backend compose file", "docker-compose.yml"},
		{"UI compose file", "compose.dev.yml"},
		{"stable compose project", "-p groupscout"},
		{"bad proxy service", "groupscout-ui-production-bad-proxy"},
	}

	for _, r := range required {
		if !strings.Contains(script, r.token) {
			t.Errorf("smoke script missing %s (expected %q)", r.label, r.token)
		}
	}
}

func TestSmokeScriptIsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows-mounted checkouts do not reliably expose POSIX executable bits")
	}

	info, err := os.Stat(scriptPath(t))
	if err != nil {
		t.Fatalf("cannot stat smoke script: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("smoke script is not executable: mode %v", info.Mode())
	}
}

func TestSmokeScriptDefaultUiRepo(t *testing.T) {
	script := readScript(t)
	if !strings.Contains(script, "groupscout-ui") {
		t.Error("smoke script does not reference the groupscout-ui repo path")
	}
}

func TestSmokeScriptHasCleanupTrap(t *testing.T) {
	script := readScript(t)
	if !strings.Contains(script, "trap cleanup EXIT") {
		t.Error("smoke script missing 'trap cleanup EXIT' - containers may not be cleaned up on failure")
	}
}

func TestSmokeScriptDoesNotRemoveVolumes(t *testing.T) {
	script := readScript(t)
	// cleanup must not pass -v flag (would remove backend data volumes)
	if strings.Contains(script, "down -v") {
		t.Error("smoke script cleanup uses 'down -v' - this would delete backend data volumes")
	}
}

func TestSmokeScriptSupportsPodmanComposeOverride(t *testing.T) {
	script := readScript(t)
	if !strings.Contains(script, `GROUPSCOUT_COMPOSE="${GROUPSCOUT_COMPOSE:-docker compose}"`) {
		t.Error("smoke script should default to Docker Compose through GROUPSCOUT_COMPOSE")
	}
	if !strings.Contains(script, `read -r -a COMPOSE <<< "${GROUPSCOUT_COMPOSE}"`) {
		t.Error("smoke script should split GROUPSCOUT_COMPOSE so 'podman compose' works")
	}
	if strings.Contains(script, "docker compose \\") {
		t.Error("smoke script should use the configurable Compose command, not hardcoded docker compose calls")
	}
}

func TestSmokeScriptUsesExplicitComposeFiles(t *testing.T) {
	script := readScript(t)
	required := []string{
		`-f "${GROUPSCOUT_BACKEND_REPO}/docker-compose.yml"`,
		`-f "${GROUPSCOUT_UI_REPO}/compose.dev.yml"`,
		`--profile smoke-ui-e2e`,
		`up -d --build groupscout-ui-production groupscout-ui-production-bad-proxy`,
		`stop groupscout-ui-production groupscout-ui-production-bad-proxy`,
		`rm -f groupscout-ui-production groupscout-ui-production-bad-proxy`,
	}

	for _, token := range required {
		if !strings.Contains(script, token) {
			t.Errorf("smoke script missing explicit Compose token %q", token)
		}
	}
}
