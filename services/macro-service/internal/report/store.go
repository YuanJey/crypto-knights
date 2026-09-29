package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("report not found")
	ErrConflict = errors.New("report already exists")
)

type Envelope struct {
	SchemaVersion string          `json:"schema_version"`
	ReportID      string          `json:"report_id"`
	GeneratedAt   time.Time       `json:"generated_at"`
	AsOf          time.Time       `json:"as_of"`
	Scope         Scope           `json:"scope"`
	DataQuality   DataQuality     `json:"data_quality"`
	Signals       []Signal        `json:"signals"`
	PositionRisks []PositionRisk  `json:"position_risks"`
	Summary       json.RawMessage `json:"summary"`
}

type Scope struct {
	Assets   []string `json:"assets"`
	Horizons []string `json:"horizons"`
}

type DataQuality struct {
	Status string `json:"status"`
}

type Signal struct {
	ID         string   `json:"id"`
	Asset      string   `json:"asset"`
	Horizon    string   `json:"horizon"`
	Direction  string   `json:"direction"`
	Score      *int     `json:"score"`
	Confidence *float64 `json:"confidence"`
}

type PositionRisk struct {
	PositionRef           string `json:"position_ref"`
	Asset                 string `json:"asset"`
	Posture               string `json:"posture"`
	Eligible              bool   `json:"eligible"`
	RequiresHumanApproval bool   `json:"requires_human_approval"`
}

type Record struct {
	Envelope Envelope        `json:"metadata"`
	Document json.RawMessage `json:"document"`
	StoredAt time.Time       `json:"stored_at"`
}

type Store struct {
	mu      sync.RWMutex
	records map[string]Record
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{
		records: make(map[string]Record),
		now:     time.Now,
	}
}

func Parse(document []byte) (Envelope, error) {
	if err := validateDocument(document); err != nil {
		return Envelope{}, err
	}
	var envelope Envelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode report: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func (e Envelope) Validate() error {
	if e.SchemaVersion != "macro-risk.v1" {
		return errors.New("schema_version must be macro-risk.v1")
	}
	if strings.TrimSpace(e.ReportID) == "" {
		return errors.New("report_id is required")
	}
	if e.GeneratedAt.IsZero() || e.AsOf.IsZero() {
		return errors.New("generated_at and as_of are required")
	}
	if e.GeneratedAt.Before(e.AsOf) {
		return errors.New("generated_at cannot be before as_of")
	}
	if len(e.Scope.Assets) == 0 || len(e.Scope.Horizons) == 0 {
		return errors.New("scope.assets and scope.horizons are required")
	}
	if !oneOf(e.DataQuality.Status, "complete", "partial", "insufficient", "stale") {
		return errors.New("invalid data_quality.status")
	}
	for index, signal := range e.Signals {
		if strings.TrimSpace(signal.ID) == "" || strings.TrimSpace(signal.Asset) == "" {
			return fmt.Errorf("signals[%d] requires id and asset", index)
		}
		if !oneOf(signal.Direction, "bearish", "neutral", "bullish", "unclear") {
			return fmt.Errorf("signals[%d] has invalid direction", index)
		}
		if signal.Score != nil && (*signal.Score < -100 || *signal.Score > 100) {
			return fmt.Errorf("signals[%d].score must be between -100 and 100", index)
		}
		if signal.Confidence != nil && (*signal.Confidence < 0 || *signal.Confidence > 1) {
			return fmt.Errorf("signals[%d].confidence must be between 0 and 1", index)
		}
	}
	for index, risk := range e.PositionRisks {
		if risk.Eligible {
			return fmt.Errorf("position_risks[%d].eligible must be false", index)
		}
		if !risk.RequiresHumanApproval {
			return fmt.Errorf("position_risks[%d].requires_human_approval must be true", index)
		}
	}
	return nil
}

func (s *Store) Create(document []byte) (Record, error) {
	envelope, err := Parse(document)
	if err != nil {
		return Record{}, err
	}

	record := Record{
		Envelope: envelope,
		Document: append(json.RawMessage(nil), document...),
		StoredAt: s.now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[envelope.ReportID]; exists {
		return Record{}, ErrConflict
	}
	s.records[envelope.ReportID] = record
	return cloneRecord(record), nil
}

func (s *Store) Get(id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, exists := s.records[id]
	if !exists {
		return Record{}, ErrNotFound
	}
	return cloneRecord(record), nil
}

func (s *Store) Latest(asset string) (Record, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))

	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		if asset == "" || contains(record.Envelope.Scope.Assets, asset) {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return Record{}, ErrNotFound
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Envelope.AsOf.After(records[j].Envelope.AsOf)
	})
	return cloneRecord(records[0]), nil
}

func (s *Store) LatestSignals(asset, horizon string) ([]Signal, error) {
	record, err := s.Latest(asset)
	if err != nil {
		return nil, err
	}

	asset = strings.ToUpper(strings.TrimSpace(asset))
	horizon = strings.TrimSpace(horizon)
	signals := make([]Signal, 0, len(record.Envelope.Signals))
	for _, signal := range record.Envelope.Signals {
		if asset != "" && strings.ToUpper(signal.Asset) != asset {
			continue
		}
		if horizon != "" && signal.Horizon != horizon {
			continue
		}
		signals = append(signals, signal)
	}
	return signals, nil
}

func cloneRecord(record Record) Record {
	record.Document = append(json.RawMessage(nil), record.Document...)
	record.Envelope.Scope.Assets = append([]string(nil), record.Envelope.Scope.Assets...)
	record.Envelope.Scope.Horizons = append([]string(nil), record.Envelope.Scope.Horizons...)
	record.Envelope.Signals = append([]Signal(nil), record.Envelope.Signals...)
	record.Envelope.PositionRisks = append([]PositionRisk(nil), record.Envelope.PositionRisks...)
	return record
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.ToUpper(value) == target {
			return true
		}
	}
	return false
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
