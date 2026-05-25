package smoke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIDockerSmokeScriptDefinesRepeatableProductionGate(t *testing.T) {
	script := readRepoFile(t, "scripts", "smoke-ui-docker-e2e.sh")

	required := []string{
		"compose config --quiet",
		"--profile smoke-ui-e2e",
		"groupscout-ui-production",
		"groupscout-ui-production-bad-proxy",
		"/healthz",
		"/assets/app.js",
		"/api/system",
		"\"401|404\" \"production UI API proxy reachability\"",
		"\"502\" \"bad-proxy UI upstream failure\"",
		"assert_compose_has_no_browser_secrets",
		"cleanup",
		"groupscout-ui-docker-e2e-ok",
	}

	for _, marker := range required {
		if !strings.Contains(script, marker) {
			t.Fatalf("script missing marker %q", marker)
		}
	}
}

func TestMakefileExposesUIDockerSmokeGate(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")

	if !strings.Contains(makefile, "smoke-ui-docker-e2e") {
		t.Fatalf("Makefile must expose smoke-ui-docker-e2e target")
	}
	if !strings.Contains(makefile, "scripts/smoke-ui-docker-e2e.sh") {
		t.Fatalf("Makefile target must run scripts/smoke-ui-docker-e2e.sh")
	}
}

func TestUIDockerSmokeScriptDoesNotInjectBrowserSecrets(t *testing.T) {
	script := readRepoFile(t, "scripts", "smoke-ui-docker-e2e.sh")
	forbiddenAssignments := []string{
		"API_TOKEN=",
		"DATABASE_URL=",
		"POSTGRES_URL=",
		"SLACK_WEBHOOK_URL=",
		"RESEND_API_KEY=",
		"SENDGRID_API_KEY=",
		"OPENAI_API_KEY=",
		"ANTHROPIC_API_KEY=",
		"CLAUDE_API_KEY=",
		"OLLAMA_BASE_URL=",
		"UI_SESSION_SECRET=",
	}

	for _, forbidden := range forbiddenAssignments {
		if strings.Contains(script, forbidden) {
			t.Fatalf("script must not inject browser-visible secret assignment %q", forbidden)
		}
	}
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}
