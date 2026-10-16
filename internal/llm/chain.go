package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Chain defaults. A run enriches a few dozen items over a few minutes, so an
// account cooldown longer than a run means one failed call per run on a dead
// account, and the provider comes back by itself once the account is fixed.
const (
	DefaultRetries           = 2
	DefaultBackoff           = 2 * time.Second
	DefaultAccountCooldown   = 6 * time.Hour
	DefaultTransientCooldown = 10 * time.Minute
	DefaultTransientTrip     = 3
)

// Chain sends each request to an ordered list of providers. Per provider:
//
//   - transient errors are retried with backoff, then the chain moves on;
//   - invalid output is retried once, then the chain moves on;
//   - request errors move on straight away;
//   - account errors move on and park the provider for AccountCooldown, so a
//     dead account costs one call, not one per item;
//   - TransientTrip requests in a row that end in transient errors park the
//     provider for TransientCooldown.
//
// A parked provider is tried again once its cooldown ends. Chain is safe for
// concurrent use.
type Chain struct {
	Retries           int
	Backoff           time.Duration
	AccountCooldown   time.Duration
	TransientCooldown time.Duration
	TransientTrip     int
	Logger            *slog.Logger

	clients []Client
	now     func() time.Time
	sleep   func(ctx context.Context, d time.Duration) error

	mu    sync.Mutex
	state map[string]*providerState
}

type providerState struct {
	parkedUntil time.Time
	reason      Kind
	lastError   string
	transients  int
}

// ProviderStatus is a provider's place in the chain and whether the chain is
// currently using it.
type ProviderStatus struct {
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Available   bool      `json:"available"`
	ParkedUntil time.Time `json:"parked_until,omitempty"`
	Reason      Kind      `json:"reason,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// ExhaustedError is returned when every provider failed or was parked.
type ExhaustedError struct {
	Errors []error
}

func (e *ExhaustedError) Error() string {
	if len(e.Errors) == 0 {
		return "no AI providers are configured"
	}
	parts := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		parts[i] = err.Error()
	}
	return "every AI provider failed: " + strings.Join(parts, "; ")
}

func (e *ExhaustedError) Unwrap() []error { return e.Errors }

// NewChain returns a chain over clients, in order, with the default policy.
func NewChain(clients ...Client) *Chain {
	c := &Chain{
		Retries:           DefaultRetries,
		Backoff:           DefaultBackoff,
		AccountCooldown:   DefaultAccountCooldown,
		TransientCooldown: DefaultTransientCooldown,
		TransientTrip:     DefaultTransientTrip,
		clients:           clients,
		now:               time.Now,
		sleep:             sleepContext,
		state:             make(map[string]*providerState),
	}
	for _, cl := range clients {
		c.state[cl.Provider()] = &providerState{}
		providerAvailable.WithLabelValues(cl.Provider()).Set(1)
	}
	return c
}

// Clients returns the providers in chain order.
func (c *Chain) Clients() []Client { return append([]Client(nil), c.clients...) }

// Complete returns the first acceptable reply from the chain.
func (c *Chain) Complete(ctx context.Context, req Request) (Response, error) {
	useCase := req.UseCase
	if useCase == "" {
		useCase = "unspecified"
	}
	var errs []error
	for _, cl := range c.clients {
		if until, parked := c.parked(cl.Provider()); parked {
			errs = append(errs, fmt.Errorf("%s: skipped until %s after earlier errors", cl.Provider(), until.Format(time.RFC3339)))
			continue
		}
		resp, err := c.try(ctx, cl, req, useCase)
		if err == nil {
			c.recordSuccess(cl)
			return resp, nil
		}
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		errs = append(errs, err)
		providerFailuresTotal.WithLabelValues(cl.Provider(), useCase, string(KindOf(err))).Inc()
		c.recordFailure(cl, err)
	}
	exhaustedTotal.WithLabelValues(useCase).Inc()
	return Response{}, &ExhaustedError{Errors: errs}
}

// try runs one provider with its retries.
func (c *Chain) try(ctx context.Context, cl Client, req Request, useCase string) (Response, error) {
	retries := 0
	retriedOutput := false
	delay := c.Backoff
	for {
		start := c.now()
		resp, err := cl.Complete(ctx, req)
		if err == nil && req.Validate != nil {
			if verr := req.Validate(resp.Text); verr != nil {
				err = &Error{Provider: cl.Provider(), Kind: KindInvalidOutput, Message: truncate(verr.Error())}
			}
		}
		requestSeconds.WithLabelValues(cl.Provider(), useCase).Observe(c.now().Sub(start).Seconds())
		if err == nil {
			requestsTotal.WithLabelValues(cl.Provider(), cl.Model(), useCase, "ok").Inc()
			return resp, nil
		}
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		kind := KindOf(err)
		requestsTotal.WithLabelValues(cl.Provider(), cl.Model(), useCase, string(kind)).Inc()

		switch {
		case kind == KindTransient && retries < c.Retries:
			retries++
			c.logger().Warn("AI provider call failed; retrying", "provider", cl.Provider(), "use_case", useCase, "attempt", retries, "error", err)
			if err := c.sleep(ctx, delay); err != nil {
				return Response{}, err
			}
			delay *= 2
		case kind == KindInvalidOutput && !retriedOutput:
			retriedOutput = true
			c.logger().Warn("AI provider reply was unusable; retrying", "provider", cl.Provider(), "use_case", useCase, "error", err)
		default:
			return Response{}, err
		}
	}
}

func (c *Chain) parked(provider string) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state[provider]
	if s == nil || s.parkedUntil.IsZero() || !c.now().Before(s.parkedUntil) {
		return time.Time{}, false
	}
	return s.parkedUntil, true
}

func (c *Chain) recordSuccess(cl Client) {
	c.mu.Lock()
	s := c.state[cl.Provider()]
	wasParked := !s.parkedUntil.IsZero()
	*s = providerState{}
	c.mu.Unlock()

	providerAvailable.WithLabelValues(cl.Provider()).Set(1)
	if wasParked {
		c.logger().Info("AI provider is working again", "provider", cl.Provider(), "model", cl.Model())
	}
}

func (c *Chain) recordFailure(cl Client, err error) {
	kind := KindOf(err)
	c.mu.Lock()
	s := c.state[cl.Provider()]
	s.lastError = truncate(err.Error())
	var cooldown time.Duration
	switch kind {
	case KindAccount:
		cooldown = c.AccountCooldown
	case KindTransient:
		s.transients++
		if s.transients >= c.TransientTrip {
			cooldown = c.TransientCooldown
		}
	default:
		s.transients = 0
	}
	if cooldown > 0 {
		s.parkedUntil = c.now().Add(cooldown)
		s.reason = kind
		s.transients = 0
	}
	until := s.parkedUntil
	c.mu.Unlock()

	if cooldown > 0 {
		providerAvailable.WithLabelValues(cl.Provider()).Set(0)
		c.logger().Error("AI provider parked; using the next provider", "provider", cl.Provider(), "model", cl.Model(),
			"reason", string(kind), "until", until.Format(time.RFC3339), "error", err)
		return
	}
	c.logger().Warn("AI provider failed this request; trying the next provider", "provider", cl.Provider(), "reason", string(kind), "error", err)
}

// Status reports every provider in chain order.
func (c *Chain) Status() []ProviderStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	out := make([]ProviderStatus, 0, len(c.clients))
	for _, cl := range c.clients {
		s := c.state[cl.Provider()]
		st := ProviderStatus{Provider: cl.Provider(), Model: cl.Model(), Available: true, LastError: s.lastError}
		if !s.parkedUntil.IsZero() && now.Before(s.parkedUntil) {
			st.Available = false
			st.ParkedUntil = s.parkedUntil
			st.Reason = s.reason
		}
		out = append(out, st)
	}
	return out
}

func (c *Chain) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// IsExhausted reports whether err means every provider failed.
func IsExhausted(err error) bool {
	var e *ExhaustedError
	return errors.As(err, &e)
}
