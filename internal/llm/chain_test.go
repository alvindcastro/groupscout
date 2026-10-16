package llm

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// scripted is a Client that replays a list of outcomes, one per call, and
// repeats the last one.
type scripted struct {
	name    string
	results []result
	calls   int
}

type result struct {
	text string
	err  error
}

func (s *scripted) Provider() string { return s.name }
func (s *scripted) Model() string    { return s.name + "-model" }

func (s *scripted) Complete(ctx context.Context, req Request) (Response, error) {
	r := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	if r.err != nil {
		return Response{}, r.err
	}
	return Response{Text: r.text, Provider: s.name, Model: s.Model()}, nil
}

func ok(text string) result { return result{text: text} }
func fail(provider string, kind Kind) result {
	return result{err: &Error{Provider: provider, Kind: kind, Message: string(kind)}}
}

// testChain returns a chain with a fake clock and no real sleeping. The
// returned pointers let a test move the clock and see the backoff delays.
func testChain(clients ...Client) (*Chain, *time.Time, *[]time.Duration) {
	c := NewChain(clients...)
	c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	var slept []time.Duration
	c.now = func() time.Time { return now }
	c.sleep = func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return c, &now, &slept
}

func TestChain_usesFirstProviderThatAnswers(t *testing.T) {
	first := &scripted{name: "t1-first", results: []result{ok("a")}}
	second := &scripted{name: "t1-second", results: []result{ok("b")}}
	c, _, _ := testChain(first, second)

	resp, err := c.Complete(context.Background(), Request{UseCase: "enrich"})
	if err != nil || resp.Text != "a" || resp.Provider != "t1-first" {
		t.Fatalf("resp %+v, err %v", resp, err)
	}
	if second.calls != 0 {
		t.Error("second provider should not be called")
	}
	if got := testutil.ToFloat64(requestsTotal.WithLabelValues("t1-first", "t1-first-model", "enrich", "ok")); got != 1 {
		t.Errorf("ok counter = %v", got)
	}
}

// The 2026-10 outage: Claude's account was empty. The chain must fail over
// to the next provider and stop calling Claude for every item.
func TestChain_accountErrorFailsOverAndParksProvider(t *testing.T) {
	claude := &scripted{name: "t2-claude", results: []result{fail("t2-claude", KindAccount)}}
	openai := &scripted{name: "t2-openai", results: []result{ok(`{"priority_score":7}`)}}
	c, now, slept := testChain(claude, openai)

	for i := 0; i < 5; i++ {
		resp, err := c.Complete(context.Background(), Request{UseCase: "enrich"})
		if err != nil || resp.Provider != "t2-openai" {
			t.Fatalf("item %d: resp %+v, err %v", i, resp, err)
		}
	}
	if claude.calls != 1 {
		t.Errorf("claude called %d times, want 1 (parked after the account error)", claude.calls)
	}
	if len(*slept) != 0 {
		t.Errorf("account errors must not be retried, slept %v", *slept)
	}
	st := c.Status()
	if st[0].Available || st[0].Reason != KindAccount || !st[0].ParkedUntil.Equal(now.Add(DefaultAccountCooldown)) || !st[1].Available {
		t.Errorf("status = %+v", st)
	}
	if got := testutil.ToFloat64(providerAvailable.WithLabelValues("t2-claude")); got != 0 {
		t.Errorf("available gauge = %v, want 0", got)
	}
	if got := testutil.ToFloat64(providerFailuresTotal.WithLabelValues("t2-claude", "enrich", "account_error")); got != 1 {
		t.Errorf("provider failures = %v, want 1", got)
	}

	// After the cooldown Claude is tried again, and comes back once it works.
	*now = now.Add(DefaultAccountCooldown)
	claude.results = []result{ok("claude is back")}
	resp, err := c.Complete(context.Background(), Request{UseCase: "enrich"})
	if err != nil || resp.Provider != "t2-claude" {
		t.Fatalf("after cooldown: resp %+v, err %v", resp, err)
	}
	if !c.Status()[0].Available || testutil.ToFloat64(providerAvailable.WithLabelValues("t2-claude")) != 1 {
		t.Error("claude should be available again")
	}
}

func TestChain_transientErrorsRetryWithBackoffThenFailOver(t *testing.T) {
	flaky := &scripted{name: "t3-flaky", results: []result{fail("t3-flaky", KindTransient)}}
	backup := &scripted{name: "t3-backup", results: []result{ok("b")}}
	c, _, slept := testChain(flaky, backup)

	resp, err := c.Complete(context.Background(), Request{})
	if err != nil || resp.Provider != "t3-backup" {
		t.Fatalf("resp %+v, err %v", resp, err)
	}
	if flaky.calls != 1+DefaultRetries {
		t.Errorf("flaky called %d times, want %d", flaky.calls, 1+DefaultRetries)
	}
	if len(*slept) != 2 || (*slept)[0] != DefaultBackoff || (*slept)[1] != 2*DefaultBackoff {
		t.Errorf("backoff = %v", *slept)
	}
	if !c.Status()[0].Available {
		t.Error("one failed request should not park a provider")
	}
}

func TestChain_transientRecoveryOnRetry(t *testing.T) {
	flaky := &scripted{name: "t4-flaky", results: []result{fail("t4-flaky", KindTransient), ok("second try")}}
	c, _, _ := testChain(flaky)

	resp, err := c.Complete(context.Background(), Request{})
	if err != nil || resp.Text != "second try" {
		t.Fatalf("resp %+v, err %v", resp, err)
	}
}

func TestChain_repeatedTransientFailuresParkProvider(t *testing.T) {
	down := &scripted{name: "t5-down", results: []result{fail("t5-down", KindTransient)}}
	backup := &scripted{name: "t5-backup", results: []result{ok("b")}}
	c, now, _ := testChain(down, backup)
	c.Retries = 0

	for i := 0; i < DefaultTransientTrip+2; i++ {
		if _, err := c.Complete(context.Background(), Request{}); err != nil {
			t.Fatal(err)
		}
	}
	if down.calls != DefaultTransientTrip {
		t.Errorf("down called %d times, want %d", down.calls, DefaultTransientTrip)
	}
	st := c.Status()[0]
	if st.Available || st.Reason != KindTransient || !st.ParkedUntil.Equal(now.Add(DefaultTransientCooldown)) {
		t.Errorf("status = %+v", st)
	}
}

func TestChain_invalidOutputRetriedOnceThenNextProvider(t *testing.T) {
	sloppy := &scripted{name: "t6-sloppy", results: []result{ok("not json")}}
	strict := &scripted{name: "t6-strict", results: []result{ok(`{"ok":true}`)}}
	c, _, slept := testChain(sloppy, strict)

	resp, err := c.Complete(context.Background(), Request{Validate: func(s string) error {
		if !strings.HasPrefix(s, "{") {
			return errors.New("not an object")
		}
		return nil
	}})
	if err != nil || resp.Provider != "t6-strict" {
		t.Fatalf("resp %+v, err %v", resp, err)
	}
	if sloppy.calls != 2 || len(*slept) != 0 {
		t.Errorf("sloppy called %d times (want 2), slept %v", sloppy.calls, *slept)
	}
	if !c.Status()[0].Available {
		t.Error("bad output should not park a provider")
	}
}

func TestChain_requestErrorMovesOnWithoutRetry(t *testing.T) {
	picky := &scripted{name: "t7-picky", results: []result{fail("t7-picky", KindRequest)}}
	backup := &scripted{name: "t7-backup", results: []result{ok("b")}}
	c, _, _ := testChain(picky, backup)

	if _, err := c.Complete(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
	if picky.calls != 1 {
		t.Errorf("picky called %d times, want 1", picky.calls)
	}
}

func TestChain_everyProviderFailing(t *testing.T) {
	a := &scripted{name: "t8-a", results: []result{fail("t8-a", KindAccount)}}
	b := &scripted{name: "t8-b", results: []result{fail("t8-b", KindRequest)}}
	c, _, _ := testChain(a, b)

	_, err := c.Complete(context.Background(), Request{UseCase: "enrich"})
	if !IsExhausted(err) {
		t.Fatalf("expected ExhaustedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "t8-a") || !strings.Contains(err.Error(), "t8-b") {
		t.Errorf("error should name both providers: %v", err)
	}
	if KindOf(err) != KindAccount {
		t.Errorf("KindOf should find the first provider error, got %s", KindOf(err))
	}

	// Second request: a is parked and reported as skipped.
	_, err = c.Complete(context.Background(), Request{UseCase: "enrich"})
	if a.calls != 1 || !strings.Contains(err.Error(), "skipped until") {
		t.Errorf("a calls %d, err %v", a.calls, err)
	}
}

func TestChain_noProviders(t *testing.T) {
	c, _, _ := testChain()
	_, err := c.Complete(context.Background(), Request{})
	if !IsExhausted(err) || !strings.Contains(err.Error(), "no AI providers") {
		t.Fatalf("err = %v", err)
	}
}

func TestChain_cancelledContextStopsWithoutFailover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := &scripted{name: "t9-first", results: []result{fail("t9-first", KindTransient)}}
	second := &scripted{name: "t9-second", results: []result{ok("b")}}
	c, _, _ := testChain(first, second)
	c.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}

	_, err := c.Complete(ctx, Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if second.calls != 0 {
		t.Error("a cancelled request must not fail over")
	}
}
