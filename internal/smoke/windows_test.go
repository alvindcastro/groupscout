package smoke_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard the Windows workstation files from IaC Phase 10
// (groupscout-site-6ah.11): the logon autostart script and the winget
// configuration.

// autostartContainers returns the container names in podman-autostart.ps1's
// default parameters, in start order.
func autostartContainers(t *testing.T) []string {
	t.Helper()
	script := readRepoFile(t, "deploy/windows/podman-autostart.ps1")
	end := strings.Index(script, "[int]$ReadyTimeoutSeconds")
	if end < 0 {
		t.Fatal("podman-autostart.ps1: parameter block not found")
	}
	var names []string
	for _, m := range regexp.MustCompile(`'(groupscout_[a-z0-9_]+)'`).FindAllStringSubmatch(script[:end], -1) {
		names = append(names, m[1])
	}
	return names
}

func TestAutostartStartsComposeContainers(t *testing.T) {
	names := autostartContainers(t)
	if len(names) == 0 || names[0] != "groupscout_postgres" {
		t.Fatalf("autostart must start groupscout_postgres first, got %v", names)
	}

	var compose struct {
		Services map[string]struct {
			ContainerName string `yaml:"container_name"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "docker-compose.yml")), &compose); err != nil {
		t.Fatal(err)
	}
	var known []string
	for _, svc := range compose.Services {
		known = append(known, svc.ContainerName)
	}
	for _, name := range names {
		if !slices.Contains(known, name) {
			t.Errorf("autostart starts %s, which docker-compose.yml does not define", name)
		}
	}
	// The cadence needs these after every reboot; monitoring needs the obs set.
	for _, required := range []string{"groupscout_app", "groupscout_n8n", "groupscout_prometheus", "groupscout_grafana"} {
		if !slices.Contains(names, required) {
			t.Errorf("autostart must start %s", required)
		}
	}
}

func TestWindowsScriptsAreASCII(t *testing.T) {
	// Windows PowerShell 5.1 reads a .ps1 without a BOM as ANSI, so any
	// non-ASCII character can corrupt a string or break parsing.
	files, err := filepath.Glob(repoPath(t, "deploy/windows/*.ps1"))
	if err != nil || len(files) < 3 {
		t.Fatalf("expected the three deploy/windows scripts, got %v (%v)", files, err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range data {
			if b > 0x7f {
				t.Errorf("%s: non-ASCII byte at offset %d", filepath.Base(f), i)
				break
			}
		}
	}
}

func TestRegisterAutostartKeepsTheTaskContract(t *testing.T) {
	script := readRepoFile(t, "deploy/windows/register-autostart.ps1")
	for _, want := range []string{
		"[string]$TaskName = 'PodmanMachineAutostart'",
		"New-ScheduledTaskTrigger -AtLogOn",
		"-RunLevel Limited",
		"-MultipleInstances IgnoreNew",
		"Register-ScheduledTask",
		"-Force",
		"SupportsShouldProcess",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("register-autostart.ps1 must contain %q", want)
		}
	}
	if strings.Contains(script, "-RunLevel Highest") {
		t.Error("the autostart task must not run elevated")
	}
}

func TestWingetConfigurationInstallsTheToolchain(t *testing.T) {
	var cfg struct {
		Properties struct {
			ConfigurationVersion string `yaml:"configurationVersion"`
			Resources            []struct {
				Resource string `yaml:"resource"`
				ID       string `yaml:"id"`
				Settings struct {
					ID     string `yaml:"id"`
					Source string `yaml:"source"`
				} `yaml:"settings"`
			} `yaml:"resources"`
		} `yaml:"properties"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/windows/configuration.dsc.yaml")), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Properties.ConfigurationVersion == "" {
		t.Error("configuration.dsc.yaml must set properties.configurationVersion")
	}
	var packages []string
	for _, r := range cfg.Properties.Resources {
		if r.Resource != "Microsoft.WinGet.DSC/WinGetPackage" || r.Settings.Source != "winget" {
			t.Errorf("resource %s must be a WinGetPackage from the winget source", r.ID)
		}
		packages = append(packages, r.Settings.ID)
	}
	for _, want := range []string{"Microsoft.WSL", "RedHat.Podman", "Docker.DockerCompose", "Git.Git", "GoLang.Go", "OpenJS.NodeJS.LTS", "FiloSottile.age", "SecretsOPerationS.SOPS"} {
		if !slices.Contains(packages, want) {
			t.Errorf("configuration.dsc.yaml must install %s", want)
		}
	}
}

func TestSetupDevNeverWritesSecrets(t *testing.T) {
	script := readRepoFile(t, "deploy/windows/setup-dev.ps1")
	if strings.Contains(script, "sops") && strings.Contains(script, "decrypt") {
		t.Error("setup-dev.ps1 must not decrypt secrets; .env comes from .env.example or make secrets-render")
	}
	if !strings.Contains(script, ".env.example") || !strings.Contains(script, "Test-Path $envFile") {
		t.Error("setup-dev.ps1 must create .env from .env.example only when .env is missing")
	}
}
