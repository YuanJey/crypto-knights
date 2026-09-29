package options

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNotFound = errors.New("option trade not found")
	ErrConflict = errors.New("option trade already exists")
)

type OptionType string

const (
	Call OptionType = "call"
	Put  OptionType = "put"
)

type AggressorSide string

const (
	Buy  AggressorSide = "buy"
	Sell AggressorSide = "sell"
)

type Trade struct {
	ID                 string          `json:"id"`
	Venue              string          `json:"venue"`
	Underlying         string          `json:"underlying"`
	Instrument         string          `json:"instrument"`
	Expiry             time.Time       `json:"expiry"`
	Strike             decimal.Decimal `json:"strike"`
	OptionType         OptionType      `json:"option_type"`
	AggressorSide      AggressorSide   `json:"aggressor_side"`
	Contracts          decimal.Decimal `json:"contracts"`
	PremiumUSD         decimal.Decimal `json:"premium_usd"`
	ContractMultiplier decimal.Decimal `json:"contract_multiplier"`
	OccurredAt         time.Time       `json:"occurred_at"`
	ReceivedAt         time.Time       `json:"received_at"`
}

func (t Trade) PremiumNotional() decimal.Decimal {
	return t.Contracts.Mul(t.PremiumUSD).Mul(t.ContractMultiplier)
}

func (t Trade) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(t.Venue) == "" ||
		strings.TrimSpace(t.Underlying) == "" ||
		strings.TrimSpace(t.Instrument) == "" {
		return errors.New("venue, underlying, and instrument are required")
	}
	if t.Expiry.IsZero() || t.OccurredAt.IsZero() {
		return errors.New("expiry and occurred_at are required")
	}
	if !t.Expiry.After(t.OccurredAt) {
		return errors.New("expiry must be after occurred_at")
	}
	if t.OptionType != Call && t.OptionType != Put {
		return errors.New("option_type must be call or put")
	}
	if t.AggressorSide != Buy && t.AggressorSide != Sell {
		return errors.New("aggressor_side must be buy or sell")
	}
	if !t.Strike.IsPositive() || !t.Contracts.IsPositive() ||
		!t.PremiumUSD.IsPositive() || !t.ContractMultiplier.IsPositive() {
		return errors.New("strike, contracts, premium_usd, and contract_multiplier must be positive")
	}
	return nil
}

type Signal struct {
	Underlying                string          `json:"underlying"`
	WindowStart               time.Time       `json:"window_start"`
	WindowEnd                 time.Time       `json:"window_end"`
	MinimumPremiumNotionalUSD decimal.Decimal `json:"minimum_premium_notional_usd"`
	BullishPremiumUSD         decimal.Decimal `json:"bullish_premium_usd"`
	BearishPremiumUSD         decimal.Decimal `json:"bearish_premium_usd"`
	NetPremiumUSD             decimal.Decimal `json:"net_premium_usd"`
	TotalPremiumUSD           decimal.Decimal `json:"total_premium_usd"`
	Direction                 string          `json:"direction"`
	Confidence                decimal.Decimal `json:"confidence"`
	TradeCount                int             `json:"trade_count"`
	TradeIDs                  []string        `json:"trade_ids"`
	KeyLevels                 []KeyLevel      `json:"key_levels"`
	GeneratedAt               time.Time       `json:"generated_at"`
}

type KeyLevel struct {
	Expiry            time.Time       `json:"expiry"`
	Strike            decimal.Decimal `json:"strike"`
	BullishPremiumUSD decimal.Decimal `json:"bullish_premium_usd"`
	BearishPremiumUSD decimal.Decimal `json:"bearish_premium_usd"`
	NetPremiumUSD     decimal.Decimal `json:"net_premium_usd"`
	TotalPremiumUSD   decimal.Decimal `json:"total_premium_usd"`
	Direction         string          `json:"direction"`
	TradeIDs          []string        `json:"trade_ids"`
}

type Store struct {
	mu                        sync.RWMutex
	trades                    map[string]Trade
	minimumPremiumNotionalUSD decimal.Decimal
	maximumKeyLevels          int
	now                       func() time.Time
}

func NewStore(minimumNotional decimal.Decimal, maximumKeyLevels int) (*Store, error) {
	if minimumNotional.IsNegative() {
		return nil, errors.New("minimum notional cannot be negative")
	}
	if maximumKeyLevels < 1 || maximumKeyLevels > 100 {
		return nil, errors.New("maximum key levels must be between 1 and 100")
	}
	return &Store{
		trades:                    make(map[string]Trade),
		minimumPremiumNotionalUSD: minimumNotional,
		maximumKeyLevels:          maximumKeyLevels,
		now:                       time.Now,
	}, nil
}

func (s *Store) Create(trade Trade) (Trade, error) {
	trade.Underlying = strings.ToUpper(strings.TrimSpace(trade.Underlying))
	trade.Instrument = strings.ToUpper(strings.TrimSpace(trade.Instrument))
	trade.Venue = strings.ToLower(strings.TrimSpace(trade.Venue))
	if err := trade.Validate(); err != nil {
		return Trade{}, err
	}
	trade.ReceivedAt = s.now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.trades[trade.ID]; exists {
		return Trade{}, ErrConflict
	}
	s.trades[trade.ID] = trade
	return trade, nil
}

func (s *Store) Get(id string) (Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trade, exists := s.trades[id]
	if !exists {
		return Trade{}, ErrNotFound
	}
	return trade, nil
}

func (s *Store) CalculateSignal(underlying string, window time.Duration) (Signal, error) {
	underlying = strings.ToUpper(strings.TrimSpace(underlying))
	if underlying == "" {
		return Signal{}, errors.New("underlying is required")
	}
	if window <= 0 {
		return Signal{}, errors.New("window must be positive")
	}

	now := s.now().UTC()
	windowStart := now.Add(-window)
	signal := Signal{
		Underlying:                underlying,
		WindowStart:               windowStart,
		WindowEnd:                 now,
		MinimumPremiumNotionalUSD: s.minimumPremiumNotionalUSD,
		BullishPremiumUSD:         decimal.Zero,
		BearishPremiumUSD:         decimal.Zero,
		NetPremiumUSD:             decimal.Zero,
		TotalPremiumUSD:           decimal.Zero,
		Direction:                 "neutral",
		Confidence:                decimal.Zero,
		GeneratedAt:               now,
	}
	levels := make(map[string]*KeyLevel)

	s.mu.RLock()
	for _, trade := range s.trades {
		if trade.Underlying != underlying ||
			trade.OccurredAt.Before(windowStart) ||
			trade.OccurredAt.After(now) {
			continue
		}
		notional := trade.PremiumNotional()
		if notional.LessThan(s.minimumPremiumNotionalUSD) {
			continue
		}
		levelKey := trade.Expiry.UTC().Format(time.RFC3339Nano) + "|" + trade.Strike.String()
		level, exists := levels[levelKey]
		if !exists {
			level = &KeyLevel{
				Expiry:            trade.Expiry.UTC(),
				Strike:            trade.Strike,
				BullishPremiumUSD: decimal.Zero,
				BearishPremiumUSD: decimal.Zero,
				NetPremiumUSD:     decimal.Zero,
				TotalPremiumUSD:   decimal.Zero,
				Direction:         "neutral",
			}
			levels[levelKey] = level
		}
		if isBullish(trade) {
			signal.BullishPremiumUSD = signal.BullishPremiumUSD.Add(notional)
			level.BullishPremiumUSD = level.BullishPremiumUSD.Add(notional)
		} else {
			signal.BearishPremiumUSD = signal.BearishPremiumUSD.Add(notional)
			level.BearishPremiumUSD = level.BearishPremiumUSD.Add(notional)
		}
		signal.TradeIDs = append(signal.TradeIDs, trade.ID)
		level.TradeIDs = append(level.TradeIDs, trade.ID)
	}
	s.mu.RUnlock()

	sort.Strings(signal.TradeIDs)
	signal.TradeCount = len(signal.TradeIDs)
	signal.TotalPremiumUSD = signal.BullishPremiumUSD.Add(signal.BearishPremiumUSD)
	signal.NetPremiumUSD = signal.BullishPremiumUSD.Sub(signal.BearishPremiumUSD)
	if signal.NetPremiumUSD.IsPositive() {
		signal.Direction = "bullish"
	} else if signal.NetPremiumUSD.IsNegative() {
		signal.Direction = "bearish"
	}
	if signal.TotalPremiumUSD.IsPositive() {
		signal.Confidence = signal.NetPremiumUSD.Abs().Div(signal.TotalPremiumUSD)
	}
	signal.KeyLevels = finalizeLevels(levels, s.maximumKeyLevels)
	return signal, nil
}

func ParseMinimumNotional(value string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse minimum notional: %w", err)
	}
	if parsed.IsNegative() {
		return decimal.Zero, errors.New("minimum notional cannot be negative")
	}
	return parsed, nil
}

func isBullish(trade Trade) bool {
	return (trade.OptionType == Call && trade.AggressorSide == Buy) ||
		(trade.OptionType == Put && trade.AggressorSide == Sell)
}

func finalizeLevels(levels map[string]*KeyLevel, maximum int) []KeyLevel {
	result := make([]KeyLevel, 0, len(levels))
	for _, level := range levels {
		sort.Strings(level.TradeIDs)
		level.TotalPremiumUSD = level.BullishPremiumUSD.Add(level.BearishPremiumUSD)
		level.NetPremiumUSD = level.BullishPremiumUSD.Sub(level.BearishPremiumUSD)
		if level.NetPremiumUSD.IsPositive() {
			level.Direction = "bullish"
		} else if level.NetPremiumUSD.IsNegative() {
			level.Direction = "bearish"
		}
		result = append(result, *level)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].TotalPremiumUSD.Equal(result[j].TotalPremiumUSD) {
			return result[i].TotalPremiumUSD.GreaterThan(result[j].TotalPremiumUSD)
		}
		if !result[i].Expiry.Equal(result[j].Expiry) {
			return result[i].Expiry.Before(result[j].Expiry)
		}
		return result[i].Strike.LessThan(result[j].Strike)
	})
	if len(result) > maximum {
		result = result[:maximum]
	}
	return result
}
