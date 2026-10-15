package smoke_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests guard secrets as code from IaC Phase 3 (groupscout-site-6ah.4):
// SOPS creation rules, encrypted env files and the secret-scan workflow.

func TestSopsRulesEncryptSecretsToAge(t *testing.T) {
	var cfg struct {
		CreationRules []struct {
			PathRegex string `yaml:"path_regex"`
			KeyGroups []struct {
				Age []string `yaml:"age"`
			} `yaml:"key_groups"`
		} `yaml:"creation_rules"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, ".sops.yaml")), &cfg); err != nil {
		t.Fatalf("parse .sops.yaml: %v", err)
	}
	ageKey := regexp.MustCompile(`^age1[0-9a-z]{58}$`)
	for _, path := range []string{"deploy/secrets/prod.sops.env", "deploy/ansible/group_vars/all/secrets.sops.yml"} {
		matched := false
		for _, rule := range cfg.CreationRules {
			if !regexp.MustCompile(rule.PathRegex).MatchString(path) {
				continue
			}
			matched = true
			n := 0
			for _, g := range rule.KeyGroups {
				for _, k := range g.Age {
					if !ageKey.MatchString(k) {
						t.Errorf("rule %q lists %q, which is not an age public key", rule.PathRegex, k)
					}
					n++
				}
			}
			if n == 0 {
				t.Errorf("rule %q has no age recipients", rule.PathRegex)
			}
			break // sops uses the first matching rule
		}
		if !matched {
			t.Errorf(".sops.yaml has no creation rule for %s", path)
		}
	}
}

// requiredProdVars lists the ${VAR:?} variables compose.prod.yml requires,
// minus the image coordinates the deploy job sets per release.
func requiredProdVars(t *testing.T) []string {
	t.Helper()
	var vars []string
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`\$\{([A-Z0-9_]+):\?`).FindAllStringSubmatch(readRepoFile(t, "deploy/compose/compose.prod.yml"), -1) {
		v := m[1]
		if seen[v] || strings.HasPrefix(v, "GROUPSCOUT_IMAGE") {
			continue
		}
		seen[v] = true
		vars = append(vars, v)
	}
	return vars
}

func TestSecretsFilesAreEncrypted(t *testing.T) {
	entries, err := os.ReadDir(repoPath(t, "deploy/secrets"))
	if err != nil {
		t.Fatalf("read deploy/secrets: %v", err)
	}
	encrypted := regexp.MustCompile(`^(sops_[a-z0-9_]+=.*|#ENC\[.*|[A-Za-z_][A-Za-z0-9_]*=(ENC\[AES256_GCM,.*)?|)$`)
	keys := map[string]map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sops.env") {
			t.Errorf("deploy/secrets/%s: only SOPS-encrypted <env>.sops.env files belong here", name)
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoPath(t, "deploy/secrets"), name))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "\nsops_mac=ENC[") || !strings.Contains(content, "\nsops_age__list_0__map_recipient=age1") {
			t.Errorf("%s has no SOPS age metadata; encrypt it with sops", name)
		}
		keys[name] = map[string]bool{}
		for i, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
			if !encrypted.MatchString(line) {
				t.Errorf("%s line %d is not encrypted", name, i+1)
				continue
			}
			if k, _, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(k, "sops_") {
				keys[name][k] = true
			}
		}
	}

	prod, ok := keys["prod.sops.env"]
	if !ok {
		t.Fatal("deploy/secrets/prod.sops.env is missing; `make secrets-render` has nothing to render")
	}
	for _, v := range requiredProdVars(t) {
		if !prod[v] {
			t.Errorf("prod.sops.env lacks %s, which compose.prod.yml requires", v)
		}
	}
}

func TestPlaintextEnvIsIgnored(t *testing.T) {
	ignore := readRepoFile(t, ".gitignore")
	for _, want := range []string{"\n.env\n", "\n.env.*\n", "\n!.env.example\n"} {
		if !strings.Contains(ignore, want) {
			t.Errorf(".gitignore must contain %q", strings.TrimSpace(want))
		}
	}
}

func TestSecretScanRunsInCI(t *testing.T) {
	wf := readRepoFile(t, ".github/workflows/secret-scan.yml")
	for _, want := range []string{"fetch-depth: 0", "sha256sum -c", "make secrets-scan"} {
		if !strings.Contains(wf, want) {
			t.Errorf("secret-scan.yml missing %q", want)
		}
	}
	for _, m := range regexp.MustCompile(`uses:\s*(\S+)`).FindAllStringSubmatch(wf, -1) {
		if !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(m[1]) {
			t.Errorf("secret-scan.yml action %s must be pinned to a commit SHA", m[1])
		}
	}

	script := readRepoFile(t, "deploy/scripts/secrets.sh")
	if !strings.Contains(script, "gitleaks git --redact") {
		t.Error("secrets.sh scan must run gitleaks with --redact (public repo and logs)")
	}
	makefile := readRepoFile(t, "Makefile")
	for _, target := range []string{"secrets-edit", "secrets-render", "secrets-diff", "secrets-updatekeys", "secrets-scan"} {
		if !strings.Contains(makefile, target) {
			t.Errorf("Makefile missing %s", target)
		}
	}
}
