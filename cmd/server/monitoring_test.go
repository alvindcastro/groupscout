package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/cadence"
	"github.com/alvindcastro/groupscout/internal/heartbeat"
	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type pingLog struct {
	mu    sync.Mutex
	paths []string
}

func (p *pingLog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, r.URL.Path)
}

func (p *pingLog) got() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
}

func vancouverTime(t *testing.T, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// newTestMonitor returns a monitor whose last cadence run was Tuesday
// 2026-10-13 18:01 and whose clock reads Thursday 2026-10-15 19:00, so the
// Thursday run is overdue until a cadence run completes.
func newTestMonitor(t *testing.T) (*opsMonitor, *pingLog) {
	t.Helper()
	pings := &pingLog{}
	srv := httptest.NewServer(pings)
	t.Cleanup(srv.Close)
	pinger, err := heartbeat.New(srv.URL + "/ping-key/groupscout-cadence")
	if err != nil {
		t.Fatal(err)
	}
	activity := storage.Activity{
		CadenceRunAt:  vancouverTime(t, "2026-10-13 18:01"),
		LeadCreatedAt: vancouverTime(t, "2026-10-13 18:00"),
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := newOpsMonitor(prometheus.NewRegistry(), cadence.Default, pinger, activity, vancouverTime(t, "2026-10-15 09:00"), log)
	m.now = func() time.Time { return vancouverTime(t, "2026-10-15 19:00") }
	return m, pings
}

func cadenceOpts() PipelineRunOptions {
	return PipelineRunOptions{GuaranteeOneLead: true, ScheduleKey: "lead-cadence:2026-10-15:thursday"}
}

func TestTrackRun_CadenceSuccessPingsAndClearsOverdue(t *testing.T) {
	m, pings := newTestMonitor(t)
	if m.cadenceOverdue() != 1 {
		t.Fatal("Thursday's run should be overdue before it completes")
	}

	_, err := m.trackRun(context.Background(), cadenceOpts(), func() (PipelineRunResult, error) {
		return PipelineRunResult{Status: "success", DeliveryStatus: "sent", Message: "sent 2 leads"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := pings.got(); len(got) != 1 || got[0] != "/ping-key/groupscout-cadence" {
		t.Errorf("pings = %v, want one success ping", got)
	}
	if m.cadenceOverdue() != 0 {
		t.Error("a completed cadence run must clear overdue")
	}
	if v := testutil.ToFloat64(m.pipelineRuns.WithLabelValues("cadence", "success")); v != 1 {
		t.Errorf("cadence success runs = %v, want 1", v)
	}
	if v := testutil.ToFloat64(m.cadenceLastRun); v != float64(vancouverTime(t, "2026-10-15 19:00").Unix()) {
		t.Errorf("cadence last run = %v", v)
	}
}

func TestTrackRun_CadenceErrorPingsFail(t *testing.T) {
	m, pings := newTestMonitor(t)
	_, err := m.trackRun(context.Background(), cadenceOpts(), func() (PipelineRunResult, error) {
		return PipelineRunResult{ScheduleKey: "lead-cadence:2026-10-15:thursday"}, errors.New("slack notify: 500")
	})
	if err == nil {
		t.Fatal("trackRun must return the run's error")
	}
	if got := pings.got(); len(got) != 1 || got[0] != "/ping-key/groupscout-cadence/fail" {
		t.Errorf("pings = %v, want one fail ping", got)
	}
	if m.cadenceOverdue() != 1 {
		t.Error("a failed run must leave the cadence overdue")
	}
	if v := testutil.ToFloat64(m.pipelineRuns.WithLabelValues("cadence", "error")); v != 1 {
		t.Errorf("cadence error runs = %v, want 1", v)
	}
}

func TestTrackRun_LockedAndAdhocRunsDoNotPing(t *testing.T) {
	m, pings := newTestMonitor(t)
	_, _ = m.trackRun(context.Background(), cadenceOpts(), func() (PipelineRunResult, error) {
		return PipelineRunResult{Status: "locked"}, nil
	})
	_, _ = m.trackRun(context.Background(), PipelineRunOptions{}, func() (PipelineRunResult, error) {
		return PipelineRunResult{Status: "success"}, nil
	})
	if got := pings.got(); len(got) != 0 {
		t.Errorf("pings = %v, want none", got)
	}
	if m.cadenceOverdue() != 1 {
		t.Error("neither a locked nor an adhoc run completes the cadence")
	}
	if testutil.ToFloat64(m.pipelineRuns.WithLabelValues("cadence", "locked")) != 1 ||
		testutil.ToFloat64(m.pipelineRuns.WithLabelValues("adhoc", "success")) != 1 {
		t.Error("locked and adhoc runs must still be counted")
	}
}

func TestTrackRun_DuplicateCountsAsCompleted(t *testing.T) {
	m, pings := newTestMonitor(t)
	_, _ = m.trackRun(context.Background(), cadenceOpts(), func() (PipelineRunResult, error) {
		return PipelineRunResult{Status: "success", DeliveryStatus: "duplicate", DeliveryDuplicate: true}, nil
	})
	if len(pings.got()) != 1 || m.cadenceOverdue() != 0 {
		t.Error("an already-delivered cadence is a completed run")
	}
	if testutil.ToFloat64(m.pipelineRuns.WithLabelValues("cadence", "duplicate")) != 1 {
		t.Error("duplicate runs must be counted as duplicate")
	}
}

func TestCollectorAndLeadMetrics(t *testing.T) {
	m, _ := newTestMonitor(t)
	m.initCollectors([]string{"richmond_permits", "vcc"})
	if n := testutil.CollectAndCount(m.collectorRuns); n != 4 {
		t.Errorf("initialized series = %d, want ok and error for each collector", n)
	}

	m.recordCollectorRun("richmond_permits", 2, nil)
	m.recordCollectorRun("vcc", 0, errors.New("selector not found"))
	if testutil.ToFloat64(m.collectorRuns.WithLabelValues("vcc", "error")) != 1 ||
		testutil.ToFloat64(m.collectorRuns.WithLabelValues("richmond_permits", "ok")) != 1 {
		t.Error("collector results not counted")
	}
	if testutil.ToFloat64(m.leadsCreated) != 2 {
		t.Error("new leads not counted")
	}
	now := float64(m.now().Unix())
	if testutil.ToFloat64(m.lastLeadCreated) != now || testutil.ToFloat64(m.collectorLastOK.WithLabelValues("richmond_permits")) != now {
		t.Error("last-lead and last-success timestamps not updated")
	}
}

func TestNewOpsMonitor_SeedsFromDatabaseOrStartTime(t *testing.T) {
	m, _ := newTestMonitor(t)
	if got := testutil.ToFloat64(m.lastLeadCreated); got != float64(vancouverTime(t, "2026-10-13 18:00").Unix()) {
		t.Errorf("last lead seeded to %v", got)
	}

	started := vancouverTime(t, "2026-10-15 09:00")
	fresh := newOpsMonitor(prometheus.NewRegistry(), cadence.Default, nil, storage.Activity{}, started, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if testutil.ToFloat64(fresh.cadenceLastRun) != float64(started.Unix()) || testutil.ToFloat64(fresh.lastLeadCreated) != float64(started.Unix()) {
		t.Error("an empty database must fall back to the start time")
	}
	// Started Thursday morning: Tuesday's slot predates the server, so nothing is overdue yet.
	fresh.now = func() time.Time { return vancouverTime(t, "2026-10-15 12:00") }
	if fresh.cadenceOverdue() != 0 {
		t.Error("a fresh install must not be overdue before its first scheduled run")
	}
}

func TestNilMonitorIsSafe(t *testing.T) {
	var m *opsMonitor
	m.initCollectors([]string{"x"})
	m.recordCollectorRun("x", 1, nil)
	m.recordLeads(3)
	res, err := m.trackRun(context.Background(), cadenceOpts(), func() (PipelineRunResult, error) {
		return PipelineRunResult{Status: "success"}, nil
	})
	if err != nil || res.Status != "success" {
		t.Errorf("nil monitor changed the run result: %+v, %v", res, err)
	}
}
