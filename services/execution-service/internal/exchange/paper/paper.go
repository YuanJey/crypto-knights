package paper

import (
	"context"
	"errors"
	"sync"

	"github.com/YuanJey/crypto-knights/services/execution-service/internal/execution"
)

type Exchange struct {
	mu       sync.Mutex
	receipts map[string]execution.ExchangeReceipt
}

func New() *Exchange {
	return &Exchange{receipts: make(map[string]execution.ExchangeReceipt)}
}

func (e *Exchange) Name() string {
	return "paper"
}

func (e *Exchange) PlaceOrder(
	ctx context.Context,
	order execution.ExchangeOrder,
) (execution.ExchangeReceipt, error) {
	if err := ctx.Err(); err != nil {
		return execution.ExchangeReceipt{}, err
	}
	if order.OrderType != "market" {
		return execution.ExchangeReceipt{}, errors.New("paper exchange supports market orders only")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if receipt, exists := e.receipts[order.ClientOrderID]; exists {
		return receipt, nil
	}
	receipt := execution.ExchangeReceipt{
		ExchangeOrderID: "paper-" + order.ClientOrderID,
		Status:          "filled",
		FilledQuantity:  order.Quantity,
		AveragePrice:    order.ReferencePrice,
	}
	e.receipts[order.ClientOrderID] = receipt
	return receipt, nil
}
