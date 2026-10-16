package smoke_test

import (
	"regexp"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// aiAlertRules is the subset of a Grafana alert rule file this test checks.
type aiAlertRules struct {
	Groups []struct {
		Rules []struct {
			UID       string `yaml:"uid"`
			Condition string `yaml:"condition"`
			Data      []struct {
				RefID         string         `yaml:"refId"`
				DatasourceUID string         `yaml:"datasourceUid"`
				Model         map[string]any `yaml:"model"`
			} `yaml:"data"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

// The AI provider alerts must query metrics internal/llm exports, through a
// provisioned datasource, so a renamed metric cannot silently disable them.
func TestAIProviderAlertRulesMatchExportedMetrics(t *testing.T) {
	var rules aiAlertRules
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/config/grafana/provisioning/alerting/llm-rules.yml")), &rules); err != nil {
		t.Fatal(err)
	}
	exported := map[string]bool{}
	for _, m := range regexp.MustCompile(`Name: "(groupscout_llm_[a-z_]+)"`).FindAllStringSubmatch(readRepoFile(t, "internal/llm/metrics.go"), -1) {
		exported[m[1]] = true
	}
	if len(exported) == 0 {
		t.Fatal("found no metrics in internal/llm/metrics.go")
	}

	var uids []string
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			uids = append(uids, r.UID)
			hasCondition := false
			for _, d := range r.Data {
				if d.DatasourceUID != "prometheus" && d.DatasourceUID != "__expr__" {
					t.Errorf("rule %s query %s uses datasource %q", r.UID, d.RefID, d.DatasourceUID)
				}
				if d.RefID == r.Condition {
					hasCondition = true
				}
				expr, _ := d.Model["expr"].(string)
				for _, m := range regexp.MustCompile(`\bgroupscout_[a-z_]+\b`).FindAllString(expr, -1) {
					if !exported[m] {
						t.Errorf("rule %s queries %s, which internal/llm/metrics.go does not export", r.UID, m)
					}
				}
			}
			if !hasCondition {
				t.Errorf("rule %s condition %q is not one of its queries", r.UID, r.Condition)
			}
		}
	}
	for _, want := range []string{"groupscout-ai-provider-account", "groupscout-ai-exhausted"} {
		if !slices.Contains(uids, want) {
			t.Errorf("llm-rules.yml lacks %s", want)
		}
	}
}

// Every provider key the app reads must reach the container: compose lists
// each variable explicitly, and AI_PROVIDER/GEMINI_API_KEY once never did.
func TestComposePassesAIProviderSettings(t *testing.T) {
	var compose struct {
		Services map[string]struct {
			Environment []string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "docker-compose.yml")), &compose); err != nil {
		t.Fatal(err)
	}
	env := compose.Services["groupscout"].Environment
	for _, want := range []string{"LLM_PROVIDERS", "CLAUDE_API_KEY", "OPENAI_API_KEY", "OPENAI_MODEL", "GEMINI_API_KEY", "CUSTOM_LLM_BASE_URL"} {
		found := false
		for _, e := range env {
			if regexp.MustCompile(`^` + want + `=`).MatchString(e) {
				found = true
			}
		}
		if !found {
			t.Errorf("docker-compose.yml groupscout service does not pass %s", want)
		}
	}
}
