package smoke_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard CI and image publishing from IaC Phase 4
// (groupscout-site-6ah.5) without needing GitHub Actions.

type workflowFile struct {
	On   map[string]any         `yaml:"on"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Needs       []string                   `yaml:"needs"`
	Permissions map[string]string          `yaml:"permissions"`
	Services    map[string]workflowService `yaml:"services"`
	Env         map[string]string          `yaml:"env"`
	Steps       []workflowStep             `yaml:"steps"`
}

type workflowService struct {
	Image string            `yaml:"image"`
	Env   map[string]string `yaml:"env"`
}

type workflowStep struct {
	Name string `yaml:"name"`
	Uses string `yaml:"uses"`
	If   string `yaml:"if"`
	Run  string `yaml:"run"`
}

func loadWorkflow(t *testing.T, rel string) workflowFile {
	t.Helper()
	var wf workflowFile
	if err := yaml.Unmarshal([]byte(readRepoFile(t, rel)), &wf); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return wf
}

// jobRun joins every run script in a job so tests can look for commands.
func jobRun(job workflowJob) string {
	var b strings.Builder
	for _, s := range job.Steps {
		b.WriteString(s.Run)
		b.WriteString("\n")
	}
	return b.String()
}

func TestWorkflowsPinActionsToCommitSHAs(t *testing.T) {
	files, err := filepath.Glob(repoPath(t, ".github/workflows/*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows found: %v", err)
	}
	sha := regexp.MustCompile(`@[0-9a-f]{40}$`)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`uses:\s*(\S+)`).FindAllStringSubmatch(string(data), -1) {
			if !strings.HasPrefix(m[1], "./") && !sha.MatchString(m[1]) {
				t.Errorf("%s: action %s must be pinned to a commit SHA", filepath.Base(f), m[1])
			}
		}
	}
}

func TestCIGatesPullRequestsAndMain(t *testing.T) {
	wf := loadWorkflow(t, ".github/workflows/ci.yml")
	if _, ok := wf.On["pull_request"]; !ok {
		t.Error("ci.yml must run on pull_request")
	}
	push, _ := wf.On["push"].(map[string]any)
	if branches, _ := push["branches"].([]any); !slices.Contains(branches, any("main")) {
		t.Error("ci.yml must run on pushes to main")
	}

	want := map[string][]string{
		"go":          {"gofmt -l", "go vet ./...", "go test", "make eval-gate"},
		"integration": {"go test -tags integration -p 1"},
		"compose":     {"make compose-check"},
		"image":       {"docker build", "compose.prod.yml", "/health", "sha256sum -c", "--only-fixed --fail-on critical", "docker push"},
	}
	for name, cmds := range want {
		job, ok := wf.Jobs[name]
		if !ok {
			t.Errorf("ci.yml has no %s job (branch protection requires it)", name)
			continue
		}
		run := jobRun(job)
		for _, c := range cmds {
			if !strings.Contains(run, c) {
				t.Errorf("ci.yml job %s must run %q", name, c)
			}
		}
	}
}

func TestCIPublishesOnlyTestedMainCommits(t *testing.T) {
	image := loadWorkflow(t, ".github/workflows/ci.yml").Jobs["image"]
	for _, dep := range []string{"go", "integration", "compose"} {
		if !slices.Contains(image.Needs, dep) {
			t.Errorf("image job must need %s so untested commits never publish", dep)
		}
	}
	if image.Env["GROUPSCOUT_IMAGE"] != "ghcr.io/alvindcastro/groupscout" || image.Env["GROUPSCOUT_IMAGE_TAG"] != "${{ github.sha }}" {
		t.Errorf("image job must tag ghcr.io/alvindcastro/groupscout with the full commit SHA, got %v", image.Env)
	}
	if image.Permissions["packages"] != "write" {
		t.Error("image job needs packages: write to push to GHCR")
	}

	scan, push := -1, -1
	for i, s := range image.Steps {
		if strings.Contains(s.Run, "--fail-on critical") {
			scan = i
		}
		if strings.Contains(s.Run, "docker push") {
			push = i
			for _, cond := range []string{"github.event_name != 'pull_request'", "github.ref == 'refs/heads/main'"} {
				if !strings.Contains(s.If, cond) {
					t.Errorf("push step must be guarded by %q", cond)
				}
			}
			if !strings.Contains(s.Run, "--password-stdin") {
				t.Error("push step must pass the registry token on stdin")
			}
		}
	}
	if scan < 0 || push < 0 || scan > push {
		t.Error("the vulnerability scan must run before the push")
	}
}

func TestCIIntegrationUsesDisposableComposePostgres(t *testing.T) {
	job := loadWorkflow(t, ".github/workflows/ci.yml").Jobs["integration"]
	svc, ok := job.Services["postgres"]
	if !ok {
		t.Fatal("integration job needs a postgres service container")
	}
	if want := loadCompose(t, "docker-compose.yml").Services["postgres"].Image; svc.Image != want {
		t.Errorf("CI postgres image %q must match docker-compose.yml (%q)", svc.Image, want)
	}

	u, err := url.Parse(job.Env["TEST_POSTGRES_URL"])
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_URL: %v", err)
	}
	// The suites delete rows between tests (groupscout-site-sfi).
	if u.Hostname() != "localhost" || strings.TrimPrefix(u.Path, "/") == "groupscout" {
		t.Errorf("TEST_POSTGRES_URL must target the throwaway service database, got host %q db %q", u.Hostname(), u.Path)
	}
	if strings.TrimPrefix(u.Path, "/") != svc.Env["POSTGRES_DB"] {
		t.Errorf("TEST_POSTGRES_URL database %q is not the service's POSTGRES_DB %q", u.Path, svc.Env["POSTGRES_DB"])
	}
}

func TestRenovateProposesPinnedGroupedUpdates(t *testing.T) {
	var cfg struct {
		Extends      []string         `json:"extends"`
		Schedule     []string         `json:"schedule"`
		Automerge    bool             `json:"automerge"`
		PackageRules []map[string]any `json:"packageRules"`
	}
	raw := readRepoFile(t, "renovate.json")
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("parse renovate.json: %v", err)
	}
	for _, preset := range []string{"config:recommended", "docker:pinDigests", "helpers:pinGitHubActionDigests"} {
		if !slices.Contains(cfg.Extends, preset) {
			t.Errorf("renovate.json must extend %s", preset)
		}
	}
	if len(cfg.Schedule) == 0 {
		t.Error("renovate.json must set a schedule so updates arrive in weekly batches")
	}
	if cfg.Automerge || strings.Contains(raw, `"automerge": true`) {
		t.Error("nothing automerges: Renovate PRs wait for review and CI")
	}

	// Images that migrate their data forward must not ride along in the
	// shared container-images group.
	for _, rule := range cfg.PackageRules {
		if rule["groupName"] != "container images" {
			continue
		}
		names, _ := rule["matchPackageNames"].([]any)
		for _, img := range []string{"n8nio/n8n", "grafana/grafana", "pgvector/pgvector"} {
			if !slices.Contains(names, any("!"+img)) {
				t.Errorf("container images group must exclude %s", img)
			}
		}
	}
}
