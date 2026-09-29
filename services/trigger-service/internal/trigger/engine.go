package trigger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNotFound      = errors.New("trigger rule not found")
	ErrConflict      = errors.New("trigger rule already exists")
	ErrInvalidStatus = errors.New("operation is not allowed in the current status")
)

type Direction string

const (
	Long  Direction = "long"
	Short Direction = "short"
)

type Status string

const (
	StatusWaiting         Status = "waiting"
	StatusTracking        Status = "tracking"
	StatusDispatchPending Status = "dispatch_pending"
	StatusDispatching     Status = "dispatching"
	StatusDispatched      Status = "dispatched"
	StatusDispatchFailed  Status = "dispatch_failed"
	StatusExpired         Status = "expired"
	StatusCanceled        Status = "canceled"
)

type Rule struct {
	ID                string           `json:"id"`
	Symbol            string           `json:"symbol"`
	Direction         Direction        `json:"direction"`
	ActivationPrice   decimal.Decimal  `json:"activation_price"`
	Retracement       decimal.Decimal  `json:"retracement"`
	WindowSeconds     int64            `json:"window_seconds"`
	Quantity          decimal.Decimal  `json:"quantity"`
	Status            Status           `json:"status"`
	CreatedAt         time.Time        `json:"created_at"`
	ActivatedAt       *time.Time       `json:"activated_at,omitempty"`
	Deadline          *time.Time       `json:"deadline,omitempty"`
	ExtremePrice      *decimal.Decimal `json:"extreme_price,omitempty"`
	LastObservedAt    *time.Time       `json:"last_observed_at,omitempty"`
	TriggeredAt       *time.Time       `json:"triggered_at,omitempty"`
	TriggerPrice      *decimal.Decimal `json:"trigger_price,omitempty"`
	DispatchAttempts  int              `json:"dispatch_attempts"`
	LastDispatchError string           `json:"last_dispatch_error,omitempty"`
	ExecutionID       string           `json:"execution_id,omitempty"`
}

func (r Rule) Validate() error {
	if r.ID != "" && strings.TrimSpace(r.ID) == "" {
		return errors.New("id cannot contain only whitespace")
	}
	if strings.TrimSpace(r.Symbol) == "" {
		return errors.New("symbol is required")
	}
	if r.Direction != Long && r.Direction != Short {
		return errors.New("direction must be long or short")
	}
	if !r.ActivationPrice.IsPositive() || !r.Retracement.IsPositive() || !r.Quantity.IsPositive() {
		return errors.New("activation_price, retracement, and quantity must be positive")
	}
	if r.WindowSeconds < 1 || r.WindowSeconds > 24*60*60 {
		return errors.New("window_seconds must be between 1 and 86400")
	}
	return nil
}

type Tick struct {
	Symbol     string          `json:"symbol"`
	Price      decimal.Decimal `json:"price"`
	ObservedAt time.Time       `json:"observed_at"`
}

func (t Tick) Validate() error {
	if strings.TrimSpace(t.Symbol) == "" {
		return errors.New("symbol is required")
	}
	if !t.Price.IsPositive() {
		return errors.New("price must be positive")
	}
	if t.ObservedAt.IsZero() {
		return errors.New("observed_at is required")
	}
	return nil
}

type ExecutionRequest struct {
	IdempotencyKey string          `json:"idempotency_key"`
	RuleID         string          `json:"rule_id"`
	Symbol         string          `json:"symbol"`
	Side           string          `json:"side"`
	OrderType      string          `json:"order_type"`
	Quantity       decimal.Decimal `json:"quantity"`
	ReferencePrice decimal.Decimal `json:"reference_price"`
	TriggeredAt    time.Time       `json:"triggered_at"`
	Reason         string          `json:"reason"`
}

type ExecutionReceipt struct {
	ExecutionID string `json:"execution_id"`
}

type Dispatcher interface {
	Submit(context.Context, ExecutionRequest) (ExecutionReceipt, error)
}

type ProcessResult struct {
	MatchedRules []Rule `json:"matched_rules"`
}

type Engine struct {
	mu         sync.RWMutex
	rules      map[string]Rule
	dispatcher Dispatcher
	now        func() time.Time
}

func NewEngine(dispatcher Dispatcher) *Engine {
	return &Engine{
		rules:      make(map[string]Rule),
		dispatcher: dispatcher,
		now:        time.Now,
	}
}

func (e *Engine) Create(rule Rule) (Rule, error) {
	rule.ID = strings.TrimSpace(rule.ID)
	rule.Symbol = strings.ToUpper(strings.TrimSpace(rule.Symbol))
	if err := rule.Validate(); err != nil {
		return Rule{}, err
	}
	if rule.ID == "" {
		id, err := newID()
		if err != nil {
			return Rule{}, fmt.Errorf("generate rule id: %w", err)
		}
		rule.ID = id
	}
	rule.Status = StatusWaiting
	rule.CreatedAt = e.now().UTC()
	rule.ActivatedAt = nil
	rule.Deadline = nil
	rule.ExtremePrice = nil
	rule.LastObservedAt = nil
	rule.TriggeredAt = nil
	rule.TriggerPrice = nil
	rule.DispatchAttempts = 0
	rule.LastDispatchError = ""
	rule.ExecutionID = ""

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.rules[rule.ID]; exists {
		return Rule{}, ErrConflict
	}
	e.rules[rule.ID] = rule
	return cloneRule(rule), nil
}

func (e *Engine) Get(id string) (Rule, error) {
	e.expire(e.now().UTC())
	e.mu.RLock()
	defer e.mu.RUnlock()
	rule, exists := e.rules[id]
	if !exists {
		return Rule{}, ErrNotFound
	}
	return cloneRule(rule), nil
}

func (e *Engine) List(symbol string) []Rule {
	e.expire(e.now().UTC())
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := make([]Rule, 0, len(e.rules))
	for _, rule := range e.rules {
		if symbol == "" || rule.Symbol == symbol {
			rules = append(rules, cloneRule(rule))
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})
	return rules
}

func (e *Engine) Cancel(id string) (Rule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rule, exists := e.rules[id]
	if !exists {
		return Rule{}, ErrNotFound
	}
	if rule.Status != StatusWaiting && rule.Status != StatusTracking &&
		rule.Status != StatusDispatchFailed {
		return Rule{}, ErrInvalidStatus
	}
	rule.Status = StatusCanceled
	e.rules[id] = rule
	return cloneRule(rule), nil
}

func (e *Engine) ProcessTick(ctx context.Context, tick Tick) (ProcessResult, error) {
	tick.Symbol = strings.ToUpper(strings.TrimSpace(tick.Symbol))
	tick.ObservedAt = tick.ObservedAt.UTC()
	if err := tick.Validate(); err != nil {
		return ProcessResult{}, err
	}

	var dispatchIDs []string
	var matchedIDs []string
	e.mu.Lock()
	for id, rule := range e.rules {
		if rule.Symbol != tick.Symbol {
			continue
		}
		if rule.Status != StatusWaiting && rule.Status != StatusTracking {
			continue
		}
		if rule.LastObservedAt != nil && !tick.ObservedAt.After(*rule.LastObservedAt) {
			continue
		}
		observedAt := tick.ObservedAt
		rule.LastObservedAt = &observedAt

		if rule.Status == StatusWaiting {
			if activationReached(rule, tick.Price) {
				activatedAt := tick.ObservedAt
				deadline := activatedAt.Add(time.Duration(rule.WindowSeconds) * time.Second)
				extreme := tick.Price
				rule.Status = StatusTracking
				rule.ActivatedAt = &activatedAt
				rule.Deadline = &deadline
				rule.ExtremePrice = &extreme
				matchedIDs = append(matchedIDs, id)
			}
			e.rules[id] = rule
			continue
		}

		if rule.Deadline == nil || tick.ObservedAt.After(*rule.Deadline) {
			rule.Status = StatusExpired
			e.rules[id] = rule
			matchedIDs = append(matchedIDs, id)
			continue
		}

		updateExtreme(&rule, tick.Price)
		if retracementReached(rule, tick.Price) {
			triggeredAt := tick.ObservedAt
			triggerPrice := tick.Price
			rule.Status = StatusDispatchPending
			rule.TriggeredAt = &triggeredAt
			rule.TriggerPrice = &triggerPrice
			dispatchIDs = append(dispatchIDs, id)
		}
		e.rules[id] = rule
		matchedIDs = append(matchedIDs, id)
	}
	e.mu.Unlock()

	for _, id := range dispatchIDs {
		_ = e.dispatch(ctx, id)
	}

	result := ProcessResult{MatchedRules: make([]Rule, 0, len(matchedIDs))}
	e.mu.RLock()
	for _, id := range matchedIDs {
		result.MatchedRules = append(result.MatchedRules, cloneRule(e.rules[id]))
	}
	e.mu.RUnlock()
	sort.Slice(result.MatchedRules, func(i, j int) bool {
		return result.MatchedRules[i].ID < result.MatchedRules[j].ID
	})
	return result, nil
}

func (e *Engine) Retry(ctx context.Context, id string) (Rule, error) {
	e.mu.RLock()
	rule, exists := e.rules[id]
	e.mu.RUnlock()
	if !exists {
		return Rule{}, ErrNotFound
	}
	if rule.Status != StatusDispatchFailed {
		return Rule{}, ErrInvalidStatus
	}
	dispatchError := e.dispatch(ctx, id)
	updated, err := e.Get(id)
	if err != nil {
		return Rule{}, err
	}
	return updated, dispatchError
}

func (e *Engine) dispatch(ctx context.Context, id string) error {
	e.mu.Lock()
	rule, exists := e.rules[id]
	if !exists {
		e.mu.Unlock()
		return ErrNotFound
	}
	if rule.Status != StatusDispatchPending && rule.Status != StatusDispatchFailed {
		e.mu.Unlock()
		return ErrInvalidStatus
	}
	if e.dispatcher == nil {
		rule.Status = StatusDispatchFailed
		rule.DispatchAttempts++
		rule.LastDispatchError = "execution dispatcher is not configured"
		e.rules[id] = rule
		e.mu.Unlock()
		return errors.New(rule.LastDispatchError)
	}

	rule.Status = StatusDispatching
	rule.DispatchAttempts++
	rule.LastDispatchError = ""
	e.rules[id] = rule
	request := executionRequest(rule)
	e.mu.Unlock()

	receipt, err := e.dispatcher.Submit(ctx, request)

	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.rules[id]
	if err != nil {
		current.Status = StatusDispatchFailed
		current.LastDispatchError = err.Error()
		e.rules[id] = current
		return err
	}
	current.Status = StatusDispatched
	current.ExecutionID = receipt.ExecutionID
	current.LastDispatchError = ""
	e.rules[id] = current
	return nil
}

func (e *Engine) expire(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, rule := range e.rules {
		if rule.Status == StatusTracking && rule.Deadline != nil && now.After(*rule.Deadline) {
			rule.Status = StatusExpired
			e.rules[id] = rule
		}
	}
}

func activationReached(rule Rule, price decimal.Decimal) bool {
	if rule.Direction == Long {
		return price.LessThanOrEqual(rule.ActivationPrice)
	}
	return price.GreaterThanOrEqual(rule.ActivationPrice)
}

func updateExtreme(rule *Rule, price decimal.Decimal) {
	if rule.ExtremePrice == nil {
		extreme := price
		rule.ExtremePrice = &extreme
		return
	}
	if rule.Direction == Long && price.LessThan(*rule.ExtremePrice) {
		extreme := price
		rule.ExtremePrice = &extreme
	}
	if rule.Direction == Short && price.GreaterThan(*rule.ExtremePrice) {
		extreme := price
		rule.ExtremePrice = &extreme
	}
}

func retracementReached(rule Rule, price decimal.Decimal) bool {
	if rule.ExtremePrice == nil {
		return false
	}
	if rule.Direction == Long {
		return price.GreaterThanOrEqual(rule.ExtremePrice.Add(rule.Retracement))
	}
	return price.LessThanOrEqual(rule.ExtremePrice.Sub(rule.Retracement))
}

func executionRequest(rule Rule) ExecutionRequest {
	side := "buy"
	if rule.Direction == Short {
		side = "sell"
	}
	return ExecutionRequest{
		IdempotencyKey: "trigger:" + rule.ID,
		RuleID:         rule.ID,
		Symbol:         rule.Symbol,
		Side:           side,
		OrderType:      "market",
		Quantity:       rule.Quantity,
		ReferencePrice: *rule.TriggerPrice,
		TriggeredAt:    *rule.TriggeredAt,
		Reason: fmt.Sprintf(
			"%s activation %s, %s-point retracement within %ds",
			rule.Direction,
			rule.ActivationPrice,
			rule.Retracement,
			rule.WindowSeconds,
		),
	}
}

func cloneRule(rule Rule) Rule {
	rule.ActivatedAt = cloneTime(rule.ActivatedAt)
	rule.Deadline = cloneTime(rule.Deadline)
	rule.LastObservedAt = cloneTime(rule.LastObservedAt)
	rule.TriggeredAt = cloneTime(rule.TriggeredAt)
	rule.ExtremePrice = cloneDecimal(rule.ExtremePrice)
	rule.TriggerPrice = cloneDecimal(rule.TriggerPrice)
	return rule
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneDecimal(value *decimal.Decimal) *decimal.Decimal {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func newID() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "rule_" + hex.EncodeToString(bytes), nil
}
