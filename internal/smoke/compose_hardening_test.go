package smoke_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard the Compose/Dockerfile hardening from IaC Phase 1
// (groupscout-site-6ah.2) without needing a container runtime.

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string         `yaml:"image"`
	Ports       []string       `yaml:"ports"`
	EnvFile     any            `yaml:"env_file"`
	Healthcheck map[string]any `yaml:"healthcheck"`
	Logging     map[string]any `yaml:"logging"`
	Profiles    []string       `yaml:"profiles"`
	DependsOn   any            `yaml:"depends_on"`
}

var coreServices = []string{"groupscout", "alertd", "postgres", "n8n"}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", rel)
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(repoPath(t, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func loadCompose(t *testing.T, rel string) composeFile {
	t.Helper()
	var c composeFile
	if err := yaml.Unmarshal([]byte(readRepoFile(t, rel)), &c); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return c
}

func TestComposeImagesArePinned(t *testing.T) {
	for _, rel := range []string{"docker-compose.yml", "deploy/compose/compose.prod.yml"} {
		for name, svc := range loadCompose(t, rel).Services {
			if svc.Image == "" || strings.Contains(svc.Image, "${") {
				continue // locally built or registry image chosen by variable
			}
			repo := svc.Image[strings.LastIndex(svc.Image, "/")+1:]
			tag := ""
			if i := strings.Index(repo, ":"); i >= 0 {
				tag = repo[i+1:]
			}
			if tag == "" || tag == "latest" {
				t.Errorf("%s: service %s image %q must pin an explicit version", rel, name, svc.Image)
			}
		}
	}
}

func TestComposeBasePublishesNoPorts(t *testing.T) {
	for name, svc := range loadCompose(t, "docker-compose.yml").Services {
		if len(svc.Ports) > 0 {
			t.Errorf("service %s publishes %v in the base file; host ports belong in docker-compose.override.yml (dev) or compose.prod.yml", name, svc.Ports)
		}
	}
}

func TestComposeProdPortsBindLoopbackOnly(t *testing.T) {
	for name, svc := range loadCompose(t, "deploy/compose/compose.prod.yml").Services {
		for _, p := range svc.Ports {
			if !strings.HasPrefix(p, "127.0.0.1:") {
				t.Errorf("prod service %s publishes %q; bind to 127.0.0.1 (Docker-published ports bypass UFW)", name, p)
			}
		}
	}
}

func TestComposeCoreServicesHaveHealthchecks(t *testing.T) {
	services := loadCompose(t, "docker-compose.yml").Services
	for _, name := range coreServices {
		svc, ok := services[name]
		if !ok {
			t.Errorf("core service %s missing from docker-compose.yml", name)
			continue
		}
		if svc.Healthcheck == nil {
			t.Errorf("core service %s has no healthcheck", name)
		}
		if len(svc.Profiles) > 0 {
			t.Errorf("core service %s must not be behind a profile, got %v", name, svc.Profiles)
		}
	}
}

func TestComposeEveryServiceRotatesLogs(t *testing.T) {
	for name, svc := range loadCompose(t, "docker-compose.yml").Services {
		opts, _ := svc.Logging["options"].(map[string]any)
		if opts["max-size"] == nil || opts["max-file"] == nil {
			t.Errorf("service %s has no log rotation (logging.options max-size/max-file)", name)
		}
	}
}

func TestComposeAlertdDoesNotReceiveWholeEnvFile(t *testing.T) {
	if svc := loadCompose(t, "docker-compose.yml").Services["alertd"]; svc.EnvFile != nil {
		t.Errorf("alertd uses env_file %v; list only the variables it reads", svc.EnvFile)
	}
}

func TestComposeOptionalServicesUseProfiles(t *testing.T) {
	services := loadCompose(t, "docker-compose.yml").Services
	want := map[string]string{
		"ollama": "llm", "ollama-init": "llm",
		"prometheus": "obs", "grafana": "obs", "loki": "obs", "promtail": "obs",
	}
	for name, profile := range want {
		if got := services[name].Profiles; len(got) != 1 || got[0] != profile {
			t.Errorf("service %s profiles = %v, want [%s]", name, got, profile)
		}
	}

	deps, _ := services["groupscout"].DependsOn.(map[string]any)
	for _, dep := range []string{"ollama", "ollama-init"} {
		d, _ := deps[dep].(map[string]any)
		if d["required"] != false {
			t.Errorf("groupscout depends_on %s must set required: false so core starts without the llm profile", dep)
		}
	}
}

func TestComposeProdRequiresSecrets(t *testing.T) {
	prod := readRepoFile(t, "deploy/compose/compose.prod.yml")
	for _, v := range []string{"GROUPSCOUT_IMAGE", "GROUPSCOUT_IMAGE_TAG", "POSTGRES_PASSWORD", "API_TOKEN", "N8N_ENCRYPTION_KEY", "GRAFANA_ADMIN_PASSWORD"} {
		if !strings.Contains(prod, "${"+v+":?") {
			t.Errorf("compose.prod.yml must require %s with ${%s:?...}", v, v)
		}
	}
}

func TestDockerfileIsPinnedAndUnprivileged(t *testing.T) {
	df := readRepoFile(t, "Dockerfile")

	pinned := regexp.MustCompile(`:\d+\.\d+\.\d+`)
	for _, line := range strings.Split(df, "\n") {
		if !strings.HasPrefix(line, "FROM ") {
			continue
		}
		if strings.Contains(line, ":latest") || !pinned.MatchString(line) {
			t.Errorf("base image must pin a full version: %q", line)
		}
	}

	if !regexp.MustCompile(`(?m)^USER [a-z]`).MatchString(df) || strings.Contains(df, "USER root") {
		t.Error("runtime stage must switch to a non-root USER")
	}

	for _, line := range strings.Split(df, "\n") {
		if strings.Contains(line, "go build") && strings.HasSuffix(strings.TrimSpace(line), ".go") {
			t.Errorf("build packages, not single files (multi-file commands break): %q", line)
		}
		if strings.Contains(line, "go build") && !strings.Contains(line, "-X main.version=") {
			t.Errorf("binaries must be stamped with the build version: %q", line)
		}
	}
}
