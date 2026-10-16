package main

// Ops metrics and the cadence heartbeat (IaC Phase 9, groupscout-site-6ah.10).
//
// The Grafana alert rules in deploy/config/grafana/provisioning/alerting read
// these metrics, and the external Healthchecks.io check receives the heartbeat.
// The last cadence run and newest lead time are seeded from the database at
// startup, so a restart cannot hide a missed run.

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alvindcastro/groupscout/internal/cadence"
	"github.com/alvindcastro/groupscout/internal/heartbeat"
	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
)

// ops is the server's monitor; nil (in tools and most tests) disables it.
var ops *opsMonitor

type opsMonitor struct {
	schedule  cadence.Schedule
	heartbeat *heartbeat.Pinger
	now       func() time.Time
	log       *slog.Logger

	mu          sync.Mutex
	lastCadence time.Time

	pipelineRuns    *prometheus.CounterVec
	collectorRuns   *prometheus.CounterVec
	collectorLastOK *prometheus.GaugeVec
	leadsCreated    prometheus.Counter
	lastLeadCreated prometheus.Gauge
	cadenceLastRun  prometheus.Gauge
}

// newOpsMonitor registers the metrics on reg. A zero time in activity (no
// cadence run or lead recorded yet, or SQLite) falls back to startedAt, so a
// fresh install doesn't alert before its first scheduled run.
func newOpsMonitor(reg prometheus.Registerer, schedule cadence.Schedule, pinger *heartbeat.Pinger, activity storage.Activity, startedAt time.Time, log *slog.Logger) *opsMonitor {
	m := &opsMonitor{
		schedule:  schedule,
		heartbeat: pinger,
		now:       time.Now,
		log:       log,
		pipelineRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_runs_total",
			Help: "Pipeline runs by kind (cadence or adhoc) and result (success, error, locked, duplicate).",
		}, []string{"kind", "result"}),
		collectorRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_collector_runs_total",
			Help: "Collector runs by collector and result (ok or error).",
		}, []string{"collector", "result"}),
		collectorLastOK: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "groupscout_collector_last_success_timestamp_seconds",
			Help: "Unix time of each collector's last successful run since the server started.",
		}, []string{"collector"}),
		leadsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "groupscout_leads_created_total",
			Help: "Leads created by pipeline runs and /ingest since the server started.",
		}),
		lastLeadCreated: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "groupscout_last_lead_created_timestamp_seconds",
			Help: "Unix time the newest lead was created, seeded from the database at startup.",
		}),
		cadenceLastRun: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "groupscout_cadence_last_run_timestamp_seconds",
			Help: "Unix time the lead cadence last completed, seeded from the database at startup.",
		}),
	}
	overdue := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "groupscout_cadence_overdue",
		Help: "1 when the most recent scheduled cadence run (Sun/Tue/Thu 18:00 America/Vancouver, plus grace) has not completed.",
	}, m.cadenceOverdue)
	reg.MustRegister(m.pipelineRuns, m.collectorRuns, m.collectorLastOK, m.leadsCreated, m.lastLeadCreated, m.cadenceLastRun, overdue)

	m.markCadenceRun(orDefault(activity.CadenceRunAt, startedAt))
	m.lastLeadCreated.Set(float64(orDefault(activity.LeadCreatedAt, startedAt).Unix()))
	return m
}

func orDefault(t, fallback time.Time) time.Time {
	if t.IsZero() {
		return fallback
	}
	return t
}

// initCollectors creates both result series for each collector at zero, so
// the first failure after a restart shows up as an increase.
func (m *opsMonitor) initCollectors(names []string) {
	if m == nil {
		return
	}
	for _, name := range names {
		m.collectorRuns.WithLabelValues(name, "ok")
		m.collectorRuns.WithLabelValues(name, "error")
	}
}

// recordCollectorRun is the enricher's OnCollectorRun hook.
func (m *opsMonitor) recordCollectorRun(name string, newLeads int, err error) {
	if m == nil {
		return
	}
	if err != nil {
		m.collectorRuns.WithLabelValues(name, "error").Inc()
	} else {
		m.collectorRuns.WithLabelValues(name, "ok").Inc()
		m.collectorLastOK.WithLabelValues(name).Set(float64(m.now().Unix()))
	}
	m.recordLeads(newLeads)
}

// recordLeads counts newly created leads.
func (m *opsMonitor) recordLeads(n int) {
	if m == nil || n <= 0 {
		return
	}
	m.leadsCreated.Add(float64(n))
	m.lastLeadCreated.Set(float64(m.now().Unix()))
}

func (m *opsMonitor) markCadenceRun(at time.Time) {
	m.mu.Lock()
	m.lastCadence = at
	m.mu.Unlock()
	m.cadenceLastRun.Set(float64(at.Unix()))
}

func (m *opsMonitor) cadenceOverdue() float64 {
	m.mu.Lock()
	last := m.lastCadence
	m.mu.Unlock()
	overdue, err := m.schedule.Overdue(m.now(), last)
	if err != nil || overdue {
		// A broken schedule must not look healthy.
		return 1
	}
	return 0
}

// trackRun runs a pipeline run and records its outcome. For cadence runs it
// also pings the heartbeat: success when the run completed, fail when it
// errored, and nothing when another run held the lock (that run pings).
func (m *opsMonitor) trackRun(ctx context.Context, opts PipelineRunOptions, run func() (PipelineRunResult, error)) (PipelineRunResult, error) {
	result, err := run()
	if m == nil {
		return result, err
	}
	kind := "adhoc"
	if opts.GuaranteeOneLead {
		kind = "cadence"
	}
	outcome := runOutcome(result, err)
	m.pipelineRuns.WithLabelValues(kind, outcome).Inc()
	if kind != "cadence" || outcome == "locked" {
		return result, err
	}

	if err != nil {
		// Keep the error itself out of the ping: it leaves the stack.
		m.ping(ctx, "fail", fmt.Sprintf("GroupScout cadence run %s failed; see the server logs.", result.ScheduleKey))
		return result, err
	}
	m.markCadenceRun(m.now())
	m.ping(ctx, "success", result.Message)
	return result, err
}

func runOutcome(result PipelineRunResult, err error) string {
	switch {
	case err != nil:
		return "error"
	case result.Status == "locked":
		return "locked"
	case result.DeliveryDuplicate:
		return "duplicate"
	default:
		return "success"
	}
}

func (m *opsMonitor) ping(ctx context.Context, signal, body string) {
	if !m.heartbeat.Enabled() {
		return
	}
	// The run's context may be nearly spent; give the ping its own budget.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	send := m.heartbeat.Success
	if signal == "fail" {
		send = m.heartbeat.Fail
	}
	if err := send(ctx, body); err != nil {
		m.log.Warn("cadence heartbeat failed", "signal", signal, "error", err)
		return
	}
	m.log.Info("cadence heartbeat sent", "signal", signal, "host", m.heartbeat.Host())
}
