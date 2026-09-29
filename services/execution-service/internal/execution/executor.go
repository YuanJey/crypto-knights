package execution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNotFound            = errors.New("execution not found")
	ErrIdempotencyConflict = errors.New("idempotency key was used with different request data")
)

type Request struct {
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

func (r Request) Validate() error {
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	if strings.TrimSpace(r.RuleID) == "" || strings.TrimSpace(r.Symbol) == "" {
		return errors.New("rule_id and symbol are required")
	}
	if r.Side != "buy" && r.Side != "sell" {
		return errors.New("side must be buy or sell")
	}
	if r.OrderType != "market" {
		return errors.New("only market orders are supported")
	}
	if !r.Quantity.IsPositive() || !r.ReferencePrice.IsPositive() {
		return errors.New("quantity and reference_price must be positive")
	}
	if r.TriggeredAt.IsZero() {
		return errors.New("triggered_at is required")
	}
	if strings.TrimSpace(r.Reason) == "" {
		return errors.New("reason is required")
	}
	return nil
}

type Policy struct {
	AllowedSymbols   map[string]struct{}
	MaxOrderNotional decimal.Decimal
	MaxTriggerAge    time.Duration
	MaxFutureSkew    time.Duration
}

func (p Policy) Validate(request Request, now time.Time) error {
	if _, allowed := p.AllowedSymbols[request.Symbol]; !allowed {
		return fmt.Errorf("symbol %s is not allowed", request.Symbol)
	}
	notional := request.Quantity.Mul(request.ReferencePrice)
	if notional.GreaterThan(p.MaxOrderNotional) {
		return fmt.Errorf(
			"reference notional %s exceeds maximum %s",
			notional,
			p.MaxOrderNotional,
		)
	}
	if request.TriggeredAt.Before(now.Add(-p.MaxTriggerAge)) {
		return errors.New("trigger is too old")
	}
	if request.TriggeredAt.After(now.Add(p.MaxFutureSkew)) {
		return errors.New("triggered_at is too far in the future")
	}
	return nil
}

type ExchangeOrder struct {
	ClientOrderID  string
	Symbol         string
	Side           string
	OrderType      string
	Quantity       decimal.Decimal
	ReferencePrice decimal.Decimal
}

type ExchangeReceipt struct {
	ExchangeOrderID string          `json:"exchange_order_id"`
	Status          string          `json:"status"`
	FilledQuantity  decimal.Decimal `json:"filled_quantity"`
	AveragePrice    decimal.Decimal `json:"average_price"`
}

type Exchange interface {
	Name() string
	PlaceOrder(context.Context, ExchangeOrder) (ExchangeReceipt, error)
}

type Record struct {
	ExecutionID     string           `json:"execution_id"`
	IdempotencyKey  string           `json:"idempotency_key"`
	RuleID          string           `json:"rule_id"`
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	OrderType       string           `json:"order_type"`
	Quantity        decimal.Decimal  `json:"quantity"`
	ReferencePrice  decimal.Decimal  `json:"reference_price"`
	TriggeredAt     time.Time        `json:"triggered_at"`
	Reason          string           `json:"reason"`
	Exchange        string           `json:"exchange"`
	Status          string           `json:"status"`
	ExchangeReceipt *ExchangeReceipt `json:"exchange_receipt,omitempty"`
	Error           string           `json:"error,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
}

type SubmitResult struct {
	Record  Record
	Created bool
}

type entry struct {
	requestHash [32]byte
	record      Record
	ready       chan struct{}
}

type Executor struct {
	mu       sync.RWMutex
	entries  map[string]*entry
	byID     map[string]*entry
	exchange Exchange
	policy   Policy
	now      func() time.Time
	newID    func() (string, error)
}

func NewExecutor(exchange Exchange, policy Policy) (*Executor, error) {
	if exchange == nil {
		return nil, errors.New("exchange is required")
	}
	if len(policy.AllowedSymbols) == 0 {
		return nil, errors.New("at least one allowed symbol is required")
	}
	if !policy.MaxOrderNotional.IsPositive() {
		return nil, errors.New("maximum order notional must be positive")
	}
	if policy.MaxTriggerAge <= 0 || policy.MaxFutureSkew < 0 {
		return nil, errors.New("invalid trigger age policy")
	}
	return &Executor{
		entries:  make(map[string]*entry),
		byID:     make(map[string]*entry),
		exchange: exchange,
		policy:   policy,
		now:      time.Now,
		newID:    newID,
	}, nil
}

func (e *Executor) Submit(ctx context.Context, request Request) (SubmitResult, error) {
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.RuleID = strings.TrimSpace(request.RuleID)
	request.Symbol = strings.ToUpper(strings.TrimSpace(request.Symbol))
	request.Reason = strings.TrimSpace(request.Reason)
	request.TriggeredAt = request.TriggeredAt.UTC()
	if err := request.Validate(); err != nil {
		return SubmitResult{}, err
	}
	if err := e.policy.Validate(request, e.now().UTC()); err != nil {
		return SubmitResult{}, err
	}

	requestHash, err := hashRequest(request)
	if err != nil {
		return SubmitResult{}, err
	}

	e.mu.Lock()
	if existing, ok := e.entries[request.IdempotencyKey]; ok {
		if existing.requestHash != requestHash {
			e.mu.Unlock()
			return SubmitResult{}, ErrIdempotencyConflict
		}
		ready := existing.ready
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return SubmitResult{}, ctx.Err()
		case <-ready:
			return SubmitResult{Record: cloneRecord(existing.record), Created: false}, nil
		}
	}

	executionID, err := e.newID()
	if err != nil {
		e.mu.Unlock()
		return SubmitResult{}, fmt.Errorf("generate execution id: %w", err)
	}
	createdAt := e.now().UTC()
	current := &entry{
		requestHash: requestHash,
		ready:       make(chan struct{}),
		record: Record{
			ExecutionID:    executionID,
			IdempotencyKey: request.IdempotencyKey,
			RuleID:         request.RuleID,
			Symbol:         request.Symbol,
			Side:           request.Side,
			OrderType:      request.OrderType,
			Quantity:       request.Quantity,
			ReferencePrice: request.ReferencePrice,
			TriggeredAt:    request.TriggeredAt,
			Reason:         request.Reason,
			Exchange:       e.exchange.Name(),
			Status:         "submitted",
			CreatedAt:      createdAt,
		},
	}
	e.entries[request.IdempotencyKey] = current
	e.byID[executionID] = current
	e.mu.Unlock()

	receipt, placementError := e.exchange.PlaceOrder(ctx, ExchangeOrder{
		ClientOrderID:  executionID,
		Symbol:         request.Symbol,
		Side:           request.Side,
		OrderType:      request.OrderType,
		Quantity:       request.Quantity,
		ReferencePrice: request.ReferencePrice,
	})

	e.mu.Lock()
	completedAt := e.now().UTC()
	if placementError != nil {
		current.record.Status = "failed"
		current.record.Error = placementError.Error()
	} else {
		current.record.Status = receipt.Status
		current.record.ExchangeReceipt = &receipt
	}
	current.record.CompletedAt = &completedAt
	close(current.ready)
	result := SubmitResult{Record: cloneRecord(current.record), Created: true}
	e.mu.Unlock()

	return result, nil
}

func (e *Executor) Get(id string) (Record, error) {
	e.mu.RLock()
	current, exists := e.byID[id]
	if !exists {
		e.mu.RUnlock()
		return Record{}, ErrNotFound
	}
	ready := current.ready
	e.mu.RUnlock()

	<-ready
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneRecord(current.record), nil
}

func (e *Executor) List() []Record {
	e.mu.RLock()
	entries := make([]*entry, 0, len(e.byID))
	for _, current := range e.byID {
		entries = append(entries, current)
	}
	e.mu.RUnlock()

	records := make([]Record, 0, len(entries))
	for _, current := range entries {
		<-current.ready
		e.mu.RLock()
		records = append(records, cloneRecord(current.record))
		e.mu.RUnlock()
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	return records
}

func ParseAllowedSymbols(value string) (map[string]struct{}, error) {
	symbols := make(map[string]struct{})
	for _, value := range strings.Split(value, ",") {
		symbol := strings.ToUpper(strings.TrimSpace(value))
		if symbol != "" {
			symbols[symbol] = struct{}{}
		}
	}
	if len(symbols) == 0 {
		return nil, errors.New("allowed symbols cannot be empty")
	}
	return symbols, nil
}

func hashRequest(request Request) ([32]byte, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return [32]byte{}, fmt.Errorf("encode request fingerprint: %w", err)
	}
	return sha256.Sum256(encoded), nil
}

func cloneRecord(record Record) Record {
	if record.ExchangeReceipt != nil {
		receipt := *record.ExchangeReceipt
		record.ExchangeReceipt = &receipt
	}
	if record.CompletedAt != nil {
		completedAt := *record.CompletedAt
		record.CompletedAt = &completedAt
	}
	return record
}

func newID() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "execution_" + hex.EncodeToString(bytes), nil
}
