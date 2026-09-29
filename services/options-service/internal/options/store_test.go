package options

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCalculateSignalFiltersLargeTradesAndClassifiesDirection(t *testing.T) {
	store, err := NewStore(decimal.RequireFromString("100000"), 10)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	trades := []Trade{
		newTrade("bull-call", Call, Buy, "2", "500", now.Add(-5*time.Minute)),
		newTrade("small-put", Put, Buy, "1", "400", now.Add(-4*time.Minute)),
		newTrade("bear-call", Call, Sell, "1", "1200", now.Add(-3*time.Minute)),
		newTrade("old-put", Put, Buy, "5", "1000", now.Add(-2*time.Hour)),
	}
	for _, trade := range trades {
		if _, err := store.Create(trade); err != nil {
			t.Fatalf("create trade %s: %v", trade.ID, err)
		}
	}

	signal, err := store.CalculateSignal("eth", time.Hour)
	if err != nil {
		t.Fatalf("calculate signal: %v", err)
	}
	if signal.Direction != "bearish" {
		t.Fatalf("direction = %s, want bearish", signal.Direction)
	}
	if !signal.BullishPremiumUSD.Equal(decimal.RequireFromString("100000")) {
		t.Fatalf("bullish premium = %s", signal.BullishPremiumUSD)
	}
	if !signal.BearishPremiumUSD.Equal(decimal.RequireFromString("120000")) {
		t.Fatalf("bearish premium = %s", signal.BearishPremiumUSD)
	}
	if !signal.NetPremiumUSD.Equal(decimal.RequireFromString("-20000")) {
		t.Fatalf("net premium = %s", signal.NetPremiumUSD)
	}
	if signal.TradeCount != 2 {
		t.Fatalf("trade count = %d, want 2", signal.TradeCount)
	}
	if len(signal.KeyLevels) != 1 ||
		!signal.KeyLevels[0].Strike.Equal(decimal.NewFromInt(3000)) {
		t.Fatalf("unexpected key levels: %#v", signal.KeyLevels)
	}
}

func TestSellPutIsBullish(t *testing.T) {
	store, err := NewStore(decimal.Zero, 10)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	if _, err := store.Create(newTrade("put-write", Put, Sell, "1", "25", now)); err != nil {
		t.Fatalf("create trade: %v", err)
	}

	signal, err := store.CalculateSignal("ETH", time.Minute)
	if err != nil {
		t.Fatalf("calculate signal: %v", err)
	}
	if signal.Direction != "bullish" || !signal.Confidence.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("unexpected signal: %#v", signal)
	}
}

func newTrade(
	id string,
	optionType OptionType,
	side AggressorSide,
	contracts string,
	premium string,
	occurredAt time.Time,
) Trade {
	return Trade{
		ID:                 id,
		Venue:              "deribit",
		Underlying:         "ETH",
		Instrument:         "ETH-20261231-3000-C",
		Expiry:             time.Date(2026, 12, 31, 8, 0, 0, 0, time.UTC),
		Strike:             decimal.NewFromInt(3000),
		OptionType:         optionType,
		AggressorSide:      side,
		Contracts:          decimal.RequireFromString(contracts),
		PremiumUSD:         decimal.RequireFromString(premium),
		ContractMultiplier: decimal.NewFromInt(100),
		OccurredAt:         occurredAt,
	}
}
