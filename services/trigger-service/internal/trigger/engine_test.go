package trigger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type fakeDispatcher struct {
	requests []ExecutionRequest
	err      error
}

func (d *fakeDispatcher) Submit(
	_ context.Context,
	request ExecutionRequest,
) (ExecutionReceipt, error) {
	d.requests = append(d.requests, request)
	if d.err != nil {
		return ExecutionReceipt{}, d.err
	}
	return ExecutionReceipt{ExecutionID: "execution-1"}, nil
}

func TestLongRuleTracksMinimumAndTriggersOnRetracement(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	engine := NewEngine(dispatcher)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	rule, err := engine.Create(newRule("long-rule", Long))
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	processPrice(t, engine, "2601", now)
	waiting := getRule(t, engine, rule.ID)
	if waiting.Status != StatusWaiting {
		t.Fatalf("status = %s, want waiting", waiting.Status)
	}

	processPrice(t, engine, "2600", now.Add(time.Second))
	processPrice(t, engine, "2592", now.Add(10*time.Second))
	processPrice(t, engine, "2601", now.Add(20*time.Second))
	tracking := getRule(t, engine, rule.ID)
	if tracking.Status != StatusTracking {
		t.Fatalf("status = %s, want tracking", tracking.Status)
	}
	if tracking.ExtremePrice == nil || !tracking.ExtremePrice.Equal(decimal.NewFromInt(2592)) {
		t.Fatalf("extreme = %v, want 2592", tracking.ExtremePrice)
	}

	processPrice(t, engine, "2602", now.Add(21*time.Second))
	dispatched := getRule(t, engine, rule.ID)
	if dispatched.Status != StatusDispatched {
		t.Fatalf("status = %s, want dispatched", dispatched.Status)
	}
	if len(dispatcher.requests) != 1 {
		t.Fatalf("dispatch count = %d, want 1", len(dispatcher.requests))
	}
	request := dispatcher.requests[0]
	if request.Side != "buy" || request.IdempotencyKey != "trigger:long-rule" {
		t.Fatalf("unexpected execution request: %#v", request)
	}
}

func TestRuleExpiresOutsideWindow(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	engine := NewEngine(dispatcher)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	rule, err := engine.Create(newRule("expires", Long))
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	processPrice(t, engine, "2600", now)
	processPrice(t, engine, "2580", now.Add(61*time.Second))
	got := getRule(t, engine, rule.ID)
	if got.Status != StatusExpired {
		t.Fatalf("status = %s, want expired", got.Status)
	}
	if len(dispatcher.requests) != 0 {
		t.Fatalf("unexpected dispatches: %d", len(dispatcher.requests))
	}
}

func TestShortRuleUsesMaximumAndFallsBack(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	engine := NewEngine(dispatcher)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	rule := newRule("short-rule", Short)
	rule.ActivationPrice = decimal.NewFromInt(3000)
	created, err := engine.Create(rule)
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	processPrice(t, engine, "3000", now)
	processPrice(t, engine, "3025", now.Add(10*time.Second))
	processPrice(t, engine, "3015", now.Add(20*time.Second))
	got := getRule(t, engine, created.ID)
	if got.Status != StatusDispatched {
		t.Fatalf("status = %s, want dispatched", got.Status)
	}
	if dispatcher.requests[0].Side != "sell" {
		t.Fatalf("side = %s, want sell", dispatcher.requests[0].Side)
	}
}

func TestFailedDispatchCanRetryWithoutDuplicateLogicalOrder(t *testing.T) {
	dispatcher := &fakeDispatcher{err: errors.New("execution unavailable")}
	engine := NewEngine(dispatcher)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	rule, err := engine.Create(newRule("retry-rule", Long))
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	processPrice(t, engine, "2600", now)
	processPrice(t, engine, "2610", now.Add(time.Second))
	failed := getRule(t, engine, rule.ID)
	if failed.Status != StatusDispatchFailed {
		t.Fatalf("status = %s, want dispatch_failed", failed.Status)
	}

	dispatcher.err = nil
	retried, err := engine.Retry(context.Background(), rule.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Status != StatusDispatched || retried.DispatchAttempts != 2 {
		t.Fatalf("unexpected retried rule: %#v", retried)
	}
	if dispatcher.requests[0].IdempotencyKey != dispatcher.requests[1].IdempotencyKey {
		t.Fatal("retry changed idempotency key")
	}
}

func newRule(id string, direction Direction) Rule {
	return Rule{
		ID:              id,
		Symbol:          "ETHUSDT",
		Direction:       direction,
		ActivationPrice: decimal.NewFromInt(2600),
		Retracement:     decimal.NewFromInt(10),
		WindowSeconds:   60,
		Quantity:        decimal.RequireFromString("0.5"),
	}
}

func processPrice(t *testing.T, engine *Engine, price string, observedAt time.Time) {
	t.Helper()
	if _, err := engine.ProcessTick(context.Background(), Tick{
		Symbol:     "ETHUSDT",
		Price:      decimal.RequireFromString(price),
		ObservedAt: observedAt,
	}); err != nil {
		t.Fatalf("process price %s: %v", price, err)
	}
}

func getRule(t *testing.T, engine *Engine, id string) Rule {
	t.Helper()
	rule, err := engine.Get(id)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	return rule
}
