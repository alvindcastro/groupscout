package alert

import (
	"context"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/aviation"
)

type mockSlack struct {
	postCalls    []AlertMessage
	updateCalls  []AlertMessage
	resolveCalls []ResolveSummary
	lastTS       string
}

func (m *mockSlack) PostMessage(ctx context.Context, msg AlertMessage) (string, error) {
	m.postCalls = append(m.postCalls, msg)
	m.lastTS = "1234567890.123456"
	return m.lastTS, nil
}

func (m *mockSlack) UpdateMessage(ctx context.Context, ts string, msg AlertMessage) error {
	m.updateCalls = append(m.updateCalls, msg)
	return nil
}

func (m *mockSlack) SendResolve(ctx context.Context, ts string, summary ResolveSummary) error {
	m.resolveCalls = append(m.resolveCalls, summary)
	return nil
}

func TestLifecycle_WatchToAlertAt30Min(t *testing.T) {
	mock := &mockSlack{}
	mgr := &LifecycleManager{
		events:   make(map[string]*DisruptionEvent),
		notifier: mock,
	}
	ctx := context.Background()
	airport := "CYVR"

	// T+0: Watch (SPS > 20)
	sps := aviation.SPSResult{State: aviation.Watch, Score: 30}
	err := mgr.Process(ctx, airport, sps, 0)
	if err != nil {
		t.Errorf("Process failed: %v", err)
	}

	if len(mock.postCalls) != 0 {
		t.Errorf("Expected 0 post calls at T+0, got %d", len(mock.postCalls))
	}

	event, ok := mgr.events[airport]
	if !ok || event.State != WatchState {
		t.Errorf("Expected event state WatchState, got %v", event.State)
	}

	// T+29: Still Watch
	event.StartedAt = time.Now().Add(-29 * time.Minute)
	sps = aviation.SPSResult{State: aviation.HardAlert, Score: 150}
	err = mgr.Process(ctx, airport, sps, 0)
	if len(mock.postCalls) != 0 {
		t.Errorf("Expected 0 post calls at T+29, got %d", len(mock.postCalls))
	}

	// T+30: Alert fires
	event.StartedAt = time.Now().Add(-30 * time.Minute)
	err = mgr.Process(ctx, airport, sps, 0)
	if len(mock.postCalls) != 1 {
		t.Errorf("Expected 1 post call at T+30, got %d", len(mock.postCalls))
	}
	if event.State != AlertState {
		t.Errorf("Expected event state AlertState, got %v", event.State)
	}
}

func TestLifecycle_UpdateOnScoreChange(t *testing.T) {
	mock := &mockSlack{}
	mgr := &LifecycleManager{
		events:   make(map[string]*DisruptionEvent),
		notifier: mock,
	}
	ctx := context.Background()
	airport := "CYVR"

	// Setup: Already in Alert state
	mgr.events[airport] = &DisruptionEvent{
		ID:        airport,
		StartedAt: time.Now().Add(-30 * time.Minute),
		State:     AlertState,
		SlackTS:   "old-ts",
		LastSPS:   aviation.SPSResult{Score: 150},
	}

	sps := aviation.SPSResult{State: aviation.HardAlert, Score: 180}
	err := mgr.Process(ctx, airport, sps, 0)
	if err != nil {
		t.Errorf("Process failed: %v", err)
	}

	if len(mock.updateCalls) != 1 {
		t.Errorf("Expected 1 update call, got %d", len(mock.updateCalls))
	}
	if len(mock.postCalls) != 0 {
		t.Errorf("Expected 0 post calls (should update), got %d", len(mock.postCalls))
	}
}

func TestLifecycle_ResolveSendsAllClear(t *testing.T) {
	mock := &mockSlack{}
	mgr := &LifecycleManager{
		events:   make(map[string]*DisruptionEvent),
		notifier: mock,
	}
	ctx := context.Background()
	airport := "CYVR"

	// Setup: Already in Alert state
	mgr.events[airport] = &DisruptionEvent{
		ID:        airport,
		StartedAt: time.Now().Add(-60 * time.Minute),
		State:     AlertState,
		SlackTS:   "some-ts",
	}

	sps := aviation.SPSResult{State: aviation.Ignore, Score: 10}
	err := mgr.Process(ctx, airport, sps, 0)
	if err != nil {
		t.Errorf("Process failed: %v", err)
	}

	if len(mock.resolveCalls) != 1 {
		t.Errorf("Expected 1 resolve call, got %d", len(mock.resolveCalls))
	}
	if mgr.events[airport].State != ResolvedState {
		t.Errorf("Expected state ResolvedState, got %v", mgr.events[airport].State)
	}
}

func TestLifecycle_NoAlertForShortFog(t *testing.T) {
	mock := &mockSlack{}
	mgr := &LifecycleManager{
		events:   make(map[string]*DisruptionEvent),
		notifier: mock,
	}
	ctx := context.Background()
	airport := "CYVR"

	// T+0: Watch
	sps := aviation.SPSResult{State: aviation.Watch, Score: 30}
	mgr.Process(ctx, airport, sps, 0)

	// T+15: Resolves (SPS goes to Ignore)
	event := mgr.events[airport]
	event.StartedAt = time.Now().Add(-15 * time.Minute)
	sps = aviation.SPSResult{State: aviation.Ignore, Score: 5}
	mgr.Process(ctx, airport, sps, 0)

	if len(mock.postCalls) != 0 {
		t.Errorf("Expected 0 post calls for short event, got %d", len(mock.postCalls))
	}
	if event.State != ResolvedState {
		t.Errorf("Expected state ResolvedState, got %v", event.State)
	}
}
