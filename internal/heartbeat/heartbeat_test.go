package heartbeat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu       sync.Mutex
	paths    []string
	bodies   []string
	failures int // respond 500 to this many requests first
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	body, _ := io.ReadAll(req.Body)
	r.paths = append(r.paths, req.Method+" "+req.URL.Path)
	r.bodies = append(r.bodies, string(body))
	if r.failures > 0 {
		r.failures--
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte("OK"))
}

func newPinger(t *testing.T, base string) *Pinger {
	t.Helper()
	p, err := New(base + "/ping-key-123/groupscout-cadence/")
	if err != nil {
		t.Fatal(err)
	}
	p.backoff = time.Millisecond
	return p
}

func TestSuccessAndFailPingTheCheck(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()
	p := newPinger(t, srv.URL)

	if err := p.Success(context.Background(), "sent 2 leads"); err != nil {
		t.Fatalf("Success: %v", err)
	}
	if err := p.Fail(context.Background(), "run failed"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	want := []string{"POST /ping-key-123/groupscout-cadence", "POST /ping-key-123/groupscout-cadence/fail"}
	if strings.Join(rec.paths, "|") != strings.Join(want, "|") {
		t.Errorf("paths = %v, want %v", rec.paths, want)
	}
	if rec.bodies[0] != "sent 2 leads" || rec.bodies[1] != "run failed" {
		t.Errorf("bodies = %q", rec.bodies)
	}
}

func TestRetriesThenSucceeds(t *testing.T) {
	rec := &recorder{failures: 2}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	if err := newPinger(t, srv.URL).Success(context.Background(), "ok"); err != nil {
		t.Fatalf("Success after two 500s: %v", err)
	}
	if len(rec.paths) != 3 {
		t.Errorf("requests = %d, want 3", len(rec.paths))
	}
}

func TestErrorsNeverContainThePingKey(t *testing.T) {
	rec := &recorder{failures: 10}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	err := newPinger(t, srv.URL).Success(context.Background(), "ok")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v, want HTTP 500", err)
	}
	if strings.Contains(err.Error(), "ping-key-123") {
		t.Errorf("error leaks the ping key: %v", err)
	}

	// A connection error must not leak it either.
	srv.Close()
	err = newPinger(t, srv.URL).Fail(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), "ping-key-123") {
		t.Errorf("connection error = %v", err)
	}
}

func TestDisabledPingerDoesNothing(t *testing.T) {
	for _, p := range []*Pinger{nil, {}} {
		if p.Enabled() || p.Success(context.Background(), "x") != nil || p.Fail(context.Background(), "x") != nil {
			t.Error("a disabled pinger must be a silent no-op")
		}
	}
	p, err := New("  ")
	if err != nil || p.Enabled() {
		t.Errorf("New(blank) = %v, %v; want disabled", p, err)
	}
}

func TestNewRejectsBadURLsWithoutEchoingThem(t *testing.T) {
	for _, bad := range []string{"hc-ping.com/secret-key", "ftp://hc-ping.com/secret-key", "https:///secret-key"} {
		_, err := New(bad)
		if err == nil {
			t.Errorf("New(%q) accepted", bad)
			continue
		}
		if strings.Contains(err.Error(), "secret-key") {
			t.Errorf("New(%q) error echoes the URL: %v", bad, err)
		}
	}
}

func TestLongBodiesAreTruncated(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	if err := newPinger(t, srv.URL).Success(context.Background(), strings.Repeat("x", maxBody+500)); err != nil {
		t.Fatal(err)
	}
	if len(rec.bodies[0]) != maxBody {
		t.Errorf("body length = %d, want %d", len(rec.bodies[0]), maxBody)
	}
}
