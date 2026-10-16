package smoke_test

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/internal/cadence"
	"gopkg.in/yaml.v3"
)

// These tests guard monitoring as code from IaC Phase 9
// (groupscout-site-6ah.10): the cadence schedule, the Healthchecks.io check,
// the Grafana alert rules and where alerts are sent.

func TestCadenceScheduleMatchesN8nAndHealthchecks(t *testing.T) {
	var wf struct {
		Settings struct {
			Timezone string `json:"timezone"`
		} `json:"settings"`
		Nodes []struct {
			Type       string `json:"type"`
			Parameters struct {
				Rule struct {
					Interval []struct {
						TriggerAtDay    []int `json:"triggerAtDay"`
						TriggerAtHour   int   `json:"triggerAtHour"`
						TriggerAtMinute int   `json:"triggerAtMinute"`
					} `json:"interval"`
				} `json:"rule"`
			} `json:"parameters"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "deploy/config/n8n/workflows/sunday-wednesday-lead-cadence.json")), &wf); err != nil {
		t.Fatal(err)
	}
	var days []int
	for _, d := range cadence.Default.Days {
		days = append(days, int(d))
	}
	found := false
	for _, n := range wf.Nodes {
		if n.Type != "n8n-nodes-base.scheduleTrigger" {
			continue
		}
		found = true
		iv := n.Parameters.Rule.Interval
		if len(iv) != 1 || !slices.Equal(iv[0].TriggerAtDay, days) || iv[0].TriggerAtHour != cadence.Default.Hour || iv[0].TriggerAtMinute != cadence.Default.Minute {
			t.Errorf("n8n schedule %+v does not match cadence.Default (days %v at %02d:%02d)", iv, days, cadence.Default.Hour, cadence.Default.Minute)
		}
	}
	if !found {
		t.Fatal("the cadence workflow has no schedule trigger")
	}
	if wf.Settings.Timezone != cadence.Default.Location {
		t.Errorf("n8n workflow timezone %q, want %q", wf.Settings.Timezone, cadence.Default.Location)
	}

	var hc struct {
		Checks []struct {
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Schedule string `json:"schedule"`
			TZ       string `json:"tz"`
			Grace    int    `json:"grace"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "deploy/config/healthchecks/checks.json")), &hc); err != nil {
		t.Fatal(err)
	}
	var dayList []string
	for _, d := range days {
		dayList = append(dayList, fmt.Sprint(d))
	}
	wantCron := fmt.Sprintf("%d %d * * %s", cadence.Default.Minute, cadence.Default.Hour, strings.Join(dayList, ","))
	for _, c := range hc.Checks {
		if c.Name != "groupscout-cadence" {
			continue
		}
		if c.Schedule != wantCron || c.TZ != cadence.Default.Location || c.Grace != int(cadence.Default.Grace.Seconds()) {
			t.Errorf("healthchecks check %+v, want schedule %q tz %q grace %v", c, wantCron, cadence.Default.Location, cadence.Default.Grace)
		}
		return
	}
	t.Error("checks.json has no groupscout-cadence check")
}

type alertRuleFile struct {
	Groups []struct {
		Folder string `yaml:"folder"`
		Rules  []struct {
			UID       string `yaml:"uid"`
			Title     string `yaml:"title"`
			Condition string `yaml:"condition"`
			Data      []struct {
				RefID         string         `yaml:"refId"`
				DatasourceUID string         `yaml:"datasourceUid"`
				Model         map[string]any `yaml:"model"`
			} `yaml:"data"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

func TestGrafanaAlertRulesAreWellFormed(t *testing.T) {
	var rules alertRuleFile
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/config/grafana/provisioning/alerting/rules.yml")), &rules); err != nil {
		t.Fatal(err)
	}
	var ds struct {
		Datasources []struct {
			UID string `yaml:"uid"`
		} `yaml:"datasources"`
	}
	if err := yaml.Unmarshal([]byte(readRepoFile(t, "deploy/config/grafana/provisioning/datasources/datasources.yml")), &ds); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"__expr__": true}
	for _, d := range ds.Datasources {
		known[d.UID] = true
	}
	// Metrics the server exports (cmd/server/monitoring.go) or Prometheus adds.
	exported := map[string]bool{"up": true}
	for _, m := range regexp.MustCompile(`Name: "(groupscout_[a-z_]+)"`).FindAllStringSubmatch(readRepoFile(t, "cmd/server/monitoring.go"), -1) {
		exported[m[1]] = true
	}

	want := []string{"groupscout-api-down", "groupscout-cadence-overdue", "groupscout-collector-failed", "groupscout-no-new-leads", "groupscout-restart-loop"}
	var got []string
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			got = append(got, r.UID)
			hasCondition := false
			for _, d := range r.Data {
				if !known[d.DatasourceUID] {
					t.Errorf("rule %s query %s uses unknown datasource %q", r.UID, d.RefID, d.DatasourceUID)
				}
				if d.RefID == r.Condition {
					hasCondition = true
				}
				expr, _ := d.Model["expr"].(string)
				if d.DatasourceUID != "prometheus" {
					continue
				}
				for _, m := range regexp.MustCompile(`\b(groupscout_[a-z_]+|up)\b`).FindAllString(expr, -1) {
					if !exported[m] {
						t.Errorf("rule %s queries %s, which cmd/server/monitoring.go does not export", r.UID, m)
					}
				}
			}
			if !hasCondition {
				t.Errorf("rule %s condition %q is not one of its queries", r.UID, r.Condition)
			}
		}
	}
	for _, uid := range want {
		if !slices.Contains(got, uid) {
			t.Errorf("rules.yml lacks %s", uid)
		}
	}
}

func TestAlertsGoToTheOpsChannelNotTheLeadChannel(t *testing.T) {
	cp := readRepoFile(t, "deploy/config/grafana/provisioning/alerting/contact-points.yml")
	if !strings.Contains(cp, "url: $GROUPSCOUT_OPS_SLACK_WEBHOOK_URL") {
		t.Error("the contact point must read the ops webhook from GROUPSCOUT_OPS_SLACK_WEBHOOK_URL")
	}
	if !strings.Contains(readRepoFile(t, "deploy/config/grafana/provisioning/alerting/policies.yml"), "receiver: groupscout-ops") {
		t.Error("the notification policy must route to groupscout-ops")
	}

	env := func(service string) []string {
		var c struct {
			Services map[string]struct {
				Environment []string `yaml:"environment"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal([]byte(readRepoFile(t, "docker-compose.yml")), &c); err != nil {
			t.Fatal(err)
		}
		return c.Services[service].Environment
	}
	grafanaOK := false
	for _, v := range env("grafana") {
		if !strings.HasPrefix(v, "GROUPSCOUT_OPS_SLACK_WEBHOOK_URL=") {
			continue
		}
		grafanaOK = strings.HasPrefix(v, "GROUPSCOUT_OPS_SLACK_WEBHOOK_URL=${OPS_SLACK_WEBHOOK_URL:-") && !strings.Contains(v, "${SLACK_WEBHOOK_URL")
		if !grafanaOK {
			t.Errorf("Grafana alerts must use OPS_SLACK_WEBHOOK_URL and never fall back to the lead channel: %s", v)
		}
	}
	if !grafanaOK && !t.Failed() {
		t.Error("grafana gets no GROUPSCOUT_OPS_SLACK_WEBHOOK_URL")
	}

	if !slices.Contains(env("groupscout"), "CADENCE_HEARTBEAT_URL=${CADENCE_HEARTBEAT_URL:-}") {
		t.Error("the API must receive CADENCE_HEARTBEAT_URL")
	}
}

func TestMonitorsScriptKeepsTheAPIKeyOffTheCommandLine(t *testing.T) {
	script := readRepoFile(t, "deploy/scripts/monitors.sh")
	if !strings.Contains(script, `-H "@$headers"`) || strings.Contains(script, `-H "X-Api-Key`) {
		t.Error("monitors.sh must pass the API key to curl in a header file, not as an argument")
	}
	if !strings.Contains(script, `chmod 600 "$headers"`) {
		t.Error("the header file must be mode 0600")
	}
	info, err := os.Stat(repoPath(t, "deploy/scripts/monitors.sh"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("monitors.sh must be executable (%v)", err)
	}
	makefile := readRepoFile(t, "Makefile")
	for _, target := range []string{"monitors-plan", "monitors-apply"} {
		if !strings.Contains(makefile, target) {
			t.Errorf("Makefile missing %s", target)
		}
	}
}
