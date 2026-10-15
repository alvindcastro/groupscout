package smoke_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard the app-config-as-code wiring from IaC Phase 2
// (groupscout-site-6ah.3): n8n workflow bootstrap, Prometheus scrape config,
// Grafana provisioning and Alloy log shipping.

func hasVolume(svc composeService, prefix string) bool {
	for _, v := range svc.Volumes {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func TestComposeBindMountSourcesExist(t *testing.T) {
	for name, svc := range loadCompose(t, "docker-compose.yml").Services {
		for _, v := range svc.Volumes {
			if !strings.HasPrefix(v, "./") {
				continue
			}
			src := strings.SplitN(v, ":", 2)[0]
			if _, err := os.Stat(repoPath(t, src)); err != nil {
				t.Errorf("service %s mounts %s, which does not exist in the repo", name, src)
			}
		}
	}
}

func TestN8nBootstrapsTrackedWorkflows(t *testing.T) {
	n8n := loadCompose(t, "docker-compose.yml").Services["n8n"]
	if strings.Join(n8n.Entrypoint, " ") != "tini -- sh /opt/groupscout/n8n/bootstrap.sh" {
		t.Errorf("n8n entrypoint = %v, want the workflow bootstrap wrapper", n8n.Entrypoint)
	}
	if !hasVolume(n8n, "./deploy/config/n8n:/opt/groupscout/n8n:ro") {
		t.Errorf("n8n must mount deploy/config/n8n read-only at /opt/groupscout/n8n, got %v", n8n.Volumes)
	}

	script := readRepoFile(t, "deploy/config/n8n/bootstrap.sh")
	if strings.Contains(script, "\r") {
		t.Error("bootstrap.sh has CRLF line endings; sh in the container cannot run it")
	}
	for _, want := range []string{"unpublish:workflow", "import:workflow", "publish:workflow", "exec /docker-entrypoint.sh \"$@\""} {
		if !strings.Contains(script, want) {
			t.Errorf("bootstrap.sh missing %q", want)
		}
	}
	for _, line := range strings.Split(script, "\n") {
		if code := strings.TrimSpace(line); !strings.HasPrefix(code, "#") && strings.Contains(code, "--activeState=fromJson") {
			t.Errorf("bootstrap.sh uses --activeState=fromJson, which n8n rejects outside queue/multi-main mode: %q", code)
		}
	}
}

func TestTrackedWorkflowsHaveStableIDs(t *testing.T) {
	files, err := filepath.Glob(repoPath(t, "deploy/config/n8n/workflows/*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no tracked workflows found (err=%v)", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Active *bool  `json:"active"`
		}
		if err := json.Unmarshal(data, &wf); err != nil {
			t.Errorf("%s is not valid JSON: %v", filepath.Base(f), err)
			continue
		}
		if wf.ID == "" || wf.Name == "" {
			t.Errorf("%s needs a stable top-level id and name (got id=%q name=%q)", filepath.Base(f), wf.ID, wf.Name)
		}
		if wf.Active == nil {
			t.Errorf("%s must declare \"active\"; the bootstrap publishes workflows marked true", filepath.Base(f))
		}
	}
}

func TestPrometheusScrapesGroupscout(t *testing.T) {
	prom := loadCompose(t, "docker-compose.yml").Services["prometheus"]
	if !hasVolume(prom, "./deploy/config/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro") {
		t.Errorf("prometheus must mount the tracked scrape config, got %v", prom.Volumes)
	}

	var cfg struct {
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			MetricsPath   string `yaml:"metrics_path"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/config/prometheus/prometheus.yml")), &cfg); err != nil {
		t.Fatalf("parse prometheus.yml: %v", err)
	}
	for _, job := range cfg.ScrapeConfigs {
		if job.JobName != "groupscout" {
			continue
		}
		for _, sc := range job.StaticConfigs {
			for _, target := range sc.Targets {
				if target == "groupscout:8080" && job.MetricsPath == "/metrics" {
					return
				}
			}
		}
	}
	t.Error("prometheus.yml must scrape groupscout:8080/metrics under job_name groupscout")
}

func TestGrafanaIsProvisioned(t *testing.T) {
	grafana := loadCompose(t, "docker-compose.yml").Services["grafana"]
	for _, want := range []string{
		"./deploy/config/grafana/provisioning:/etc/grafana/provisioning:ro",
		"./deploy/config/grafana/dashboards:/etc/grafana/dashboards:ro",
	} {
		if !hasVolume(grafana, want) {
			t.Errorf("grafana must mount %s", want)
		}
	}

	var ds struct {
		Datasources []struct {
			UID string `yaml:"uid"`
			URL string `yaml:"url"`
		} `yaml:"datasources"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/config/grafana/provisioning/datasources/datasources.yml")), &ds); err != nil {
		t.Fatalf("parse datasources.yml: %v", err)
	}
	uids := map[string]bool{}
	for _, d := range ds.Datasources {
		uids[d.UID] = true
	}
	for _, want := range []string{"prometheus", "loki"} {
		if !uids[want] {
			t.Errorf("datasources.yml missing uid %q", want)
		}
	}

	dashboards, _ := filepath.Glob(repoPath(t, "deploy/config/grafana/dashboards/*.json"))
	if len(dashboards) == 0 {
		t.Fatal("no provisioned dashboards found")
	}
	for _, f := range dashboards {
		data, _ := os.ReadFile(f)
		var dash struct {
			UID    string `json:"uid"`
			Panels []struct {
				Title      string `json:"title"`
				Datasource struct {
					UID string `json:"uid"`
				} `json:"datasource"`
			} `json:"panels"`
		}
		if err := json.Unmarshal(data, &dash); err != nil {
			t.Errorf("%s is not valid JSON: %v", filepath.Base(f), err)
			continue
		}
		if dash.UID == "" {
			t.Errorf("%s needs a stable uid so links survive reprovisioning", filepath.Base(f))
		}
		for _, p := range dash.Panels {
			if !uids[p.Datasource.UID] {
				t.Errorf("%s panel %q uses datasource uid %q, which is not provisioned", filepath.Base(f), p.Title, p.Datasource.UID)
			}
		}
	}
}

func TestAlloyShipsContainerLogsToLoki(t *testing.T) {
	services := loadCompose(t, "docker-compose.yml").Services
	if _, ok := services["promtail"]; ok {
		t.Error("promtail is deprecated and never worked on Podman; alloy replaces it")
	}
	alloy, ok := services["alloy"]
	if !ok {
		t.Fatal("alloy service missing")
	}
	for _, want := range []string{
		"./deploy/config/alloy/config.alloy:/etc/alloy/config.alloy:ro",
		// The host side is a variable so rootless Podman hosts can mount the
		// user's socket; Docker and the Podman machine keep the default.
		"${CONTAINER_SOCKET:-/var/run/docker.sock}:/var/run/docker.sock:ro",
	} {
		if !hasVolume(alloy, want) {
			t.Errorf("alloy must mount %s", want)
		}
	}

	cfg := readRepoFile(t, "deploy/config/alloy/config.alloy")
	for _, want := range []string{"discovery.docker", "loki.source.docker", "http://loki:3100/loki/api/v1/push", `target_label  = "container"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.alloy missing %q", want)
		}
	}
}
