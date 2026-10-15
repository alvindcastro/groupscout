package smoke_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard host configuration from IaC Phase 5
// (groupscout-site-6ah.6): deploy/ansible and the WSL host bootstrap.

type ansibleVars struct {
	SecretsEnv string         `yaml:"groupscout_secrets_env"`
	Services   []string       `yaml:"groupscout_services"`
	Profiles   []string       `yaml:"groupscout_compose_profiles"`
	HostPorts  map[string]int `yaml:"groupscout_host_ports"`
	ImageTag   string         `yaml:"groupscout_image_tag"`
}

func loadAnsibleVars(t *testing.T, rel string) ansibleVars {
	t.Helper()
	var v ansibleVars
	if err := yaml.Unmarshal([]byte(readRepoFile(t, rel)), &v); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return v
}

func TestAnsibleStagingCannotRunTheCadenceOrAlertd(t *testing.T) {
	group := loadAnsibleVars(t, "deploy/ansible/group_vars/all/main.yml")
	host := loadAnsibleVars(t, "deploy/ansible/inventory/host_vars/groupscout-prod.yml")

	if group.SecretsEnv != "prod" || !slices.Contains(group.Services, "n8n") {
		t.Errorf("the defaults must describe production: prod secrets and n8n, got %+v", group)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(group.ImageTag) {
		t.Errorf("deploy the image by full commit SHA, got %q", group.ImageTag)
	}
	for _, v := range []ansibleVars{group, host} {
		// alertd posts to the hotel Slack channel with the real bot token
		// (groupscout-site-q74).
		if slices.Contains(v.Services, "alertd") {
			t.Error("alertd must stay out of groupscout_services until groupscout-site-q74 decides")
		}
	}
	// Whatever the host overrides, a staging stack must not start n8n: its
	// bootstrap publishes the cadence workflow, and two cadences would send
	// every lead twice.
	if host.SecretsEnv == "staging" && slices.Contains(host.Services, "n8n") {
		t.Error("a staging host must not run n8n")
	}
	if host.SecretsEnv == "staging" && host.HostPorts["api"] == 8080 {
		t.Error("staging must use ports beside the live stack's 8080")
	}
}

func TestAnsibleCollectionsArePinned(t *testing.T) {
	var req struct {
		Collections []struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"collections"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/ansible/requirements.yml")), &req); err != nil {
		t.Fatal(err)
	}
	if len(req.Collections) == 0 {
		t.Fatal("requirements.yml lists no collections")
	}
	for _, c := range req.Collections {
		if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(c.Version) {
			t.Errorf("collection %s must pin an exact version, got %q", c.Name, c.Version)
		}
	}
}

func TestAnsibleRendersSecretsSafely(t *testing.T) {
	tasks := readRepoFile(t, "deploy/ansible/roles/groupscout/tasks/main.yml")
	i := strings.Index(tasks, "community.sops.sops")
	if i < 0 {
		t.Fatal("the groupscout role must read secrets through the community.sops lookup")
	}
	block := tasks[strings.LastIndex(tasks[:i], "- name:"):]
	block = block[:strings.Index(block[1:], "\n- name:")+1]
	for _, want := range []string{`mode: "0600"`, "no_log: true"} {
		if !strings.Contains(block, want) {
			t.Errorf("the .env task must set %s", want)
		}
	}

	env := readRepoFile(t, "deploy/ansible/roles/groupscout/templates/env.j2")
	for _, want := range []string{"GROUPSCOUT_IMAGE_TAG=", "CONTAINER_SOCKET=/run/user/", "GROUPSCOUT_API_HOST_PORT=", "COMPOSE_PROFILES="} {
		if !strings.Contains(env, want) {
			t.Errorf("env.j2 must set %s", want)
		}
	}
	if !strings.Contains(tasks, "deploy/compose/compose.prod.yml") {
		t.Error("the stack must run with the prod overlay")
	}
}

func TestComposeProdPortsDefaultToTheStandardPorts(t *testing.T) {
	prod := readRepoFile(t, "deploy/compose/compose.prod.yml")
	for _, want := range []string{
		`"127.0.0.1:${GROUPSCOUT_API_HOST_PORT:-8080}:8080"`,
		`"127.0.0.1:${GROUPSCOUT_N8N_HOST_PORT:-5678}:5678"`,
		`"127.0.0.1:${GROUPSCOUT_GRAFANA_HOST_PORT:-3000}:3000"`,
	} {
		if !strings.Contains(prod, want) {
			t.Errorf("compose.prod.yml must publish %s", want)
		}
	}
}

func TestWSLHostAvoidsStateSharedBetweenDistros(t *testing.T) {
	// WSL distros share one cgroup tree and one binfmt_misc table.
	bootstrap := readRepoFile(t, "deploy/windows/new-wsl-host.ps1")
	m := regexp.MustCompile(`\[int\]\$DeployUid = (\d+)`).FindStringSubmatch(bootstrap)
	if m == nil || m[1] == "1000" {
		t.Errorf("new-wsl-host.ps1 must give the deploy user a uid other than 1000, got %v", m)
	}
	for _, want := range []string{"enabled=false", "wsl-binfmt-detach.service", "umount -l /proc/sys/fs/binfmt_misc"} {
		if !strings.Contains(bootstrap, want) {
			t.Errorf("new-wsl-host.ps1 must contain %q", want)
		}
	}

	if !strings.Contains(readRepoFile(t, "deploy/ansible/site.yml"), "deploy_uid | int != 1000") {
		t.Error("site.yml must refuse a uid-1000 deploy user on WSL")
	}
	base := readRepoFile(t, "deploy/ansible/roles/base/tasks/main.yml")
	for _, unit := range []string{"systemd-binfmt.service", "proc-sys-fs-binfmt_misc.automount", "proc-sys-fs-binfmt_misc.mount", "wsl-binfmt-detach.service"} {
		if !strings.Contains(base, unit) {
			t.Errorf("the base role must manage %s on WSL", unit)
		}
	}
	if !strings.Contains(readRepoFile(t, "deploy/ansible/roles/base/templates/wsl.conf.j2"), "enabled=false") {
		t.Error("wsl.conf must turn Windows interop off on the host")
	}
}

func TestCILintsTheAnsible(t *testing.T) {
	job, ok := loadWorkflow(t, ".github/workflows/ci.yml").Jobs["ansible"]
	if !ok {
		t.Fatal("ci.yml has no ansible job")
	}
	run := jobRun(job)
	for _, want := range []string{"ansible-core==2.21.4", "ansible-lint==", "make ansible-deps", "make ansible-lint", "--syntax-check"} {
		if !strings.Contains(run, want) {
			t.Errorf("ansible job must run %q", want)
		}
	}
}
