package smoke

import (
	"os"
	"strings"
	"testing"
)

func TestUIDockerSmokeScriptContract(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/smoke-ui-docker-e2e.sh")
	if err != nil {
		t.Fatalf("read smoke script: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		"GROUPSCOUT_UI_REPO",
		"docker compose -p groupscout",
		"docker build --target production",
		"/healthz",
		"/assets/app.js",
		"/api/leads?limit=1",
		"backend 404",
		"proxy 502",
		"node --test",
		"lead-inbox-screen.test.js",
		"lead-detail-screen.test.js",
		"API_TOKEN",
		"DATABASE_URL",
		"SLACK_WEBHOOK_URL",
		"RESEND_API_KEY",
		"CLAUDE_API_KEY",
		"UI_SESSION_SECRET",
		"http://groupscout:8080",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
}
