package execution_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/YuanJey/crypto-knights/services/execution-service/internal/exchange/paper"
	"github.com/YuanJey/crypto-knights/services/execution-service/internal/execution"
)

func TestSubmitIsIdempotent(t *testing.T) {
	executor := newExecutor(t)
	request := validRequest()

	first, err := executor.Submit(context.Background(), request)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := executor.Submit(context.Background(), request)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if !first.Created || second.Created {
		t.Fatalf("created flags = %v, %v", first.Created, second.Created)
	}
	if first.Record.ExecutionID != second.Record.ExecutionID {
		t.Fatalf("execution IDs differ: %s != %s", first.Record.ExecutionID, second.Record.ExecutionID)
	}
	if first.Record.Status != "filled" {
		t.Fatalf("status = %s, want filled", first.Record.Status)
	}
}

func TestConcurrentSubmitCreatesOneExecution(t *testing.T) {
	executor := newExecutor(t)
	request := validRequest()

	var wait sync.WaitGroup
	results := make(chan execution.SubmitResult, 8)
	errorsChannel := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := executor.Submit(context.Background(), request)
			results <- result
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errorsChannel)

	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	executionIDs := make(map[string]struct{})
	createdCount := 0
	for result := range results {
		executionIDs[result.Record.ExecutionID] = struct{}{}
		if result.Created {
			createdCount++
		}
	}
	if len(executionIDs) != 1 || createdCount != 1 {
		t.Fatalf("execution IDs = %d, created count = %d", len(executionIDs), createdCount)
	}
}

func TestRejectsChangedRequestForSameIdempotencyKey(t *testing.T) {
	executor := newExecutor(t)
	request := validRequest()
	if _, err := executor.Submit(context.Background(), request); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	request.Quantity = decimal.NewFromInt(2)
	if _, err := executor.Submit(context.Background(), request); !errors.Is(
		err,
		execution.ErrIdempotencyConflict,
	) {
		t.Fatalf("error = %v, want idempotency conflict", err)
	}
}

func TestPolicyRejectsExcessNotionalAndStaleTrigger(t *testing.T) {
	executor := newExecutor(t)
	tooLarge := validRequest()
	tooLarge.Quantity = decimal.NewFromInt(10)
	if _, err := executor.Submit(context.Background(), tooLarge); err == nil {
		t.Fatal("expected excessive notional to be rejected")
	}

	stale := validRequest()
	stale.IdempotencyKey = "stale"
	stale.TriggeredAt = time.Now().Add(-time.Hour)
	if _, err := executor.Submit(context.Background(), stale); err == nil {
		t.Fatal("expected stale trigger to be rejected")
	}
}

func newExecutor(t *testing.T) *execution.Executor {
	t.Helper()
	executor, err := execution.NewExecutor(paper.New(), execution.Policy{
		AllowedSymbols: map[string]struct{}{
			"ETHUSDT": {},
		},
		MaxOrderNotional: decimal.NewFromInt(10000),
		MaxTriggerAge:    time.Minute,
		MaxFutureSkew:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	return executor
}

func validRequest() execution.Request {
	return execution.Request{
		IdempotencyKey: "trigger:rule-1",
		RuleID:         "rule-1",
		Symbol:         "ETHUSDT",
		Side:           "buy",
		OrderType:      "market",
		Quantity:       decimal.RequireFromString("0.5"),
		ReferencePrice: decimal.NewFromInt(2600),
		TriggeredAt:    time.Now().UTC(),
		Reason:         "test trigger",
	}
}
