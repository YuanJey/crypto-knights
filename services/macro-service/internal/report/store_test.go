package report

import (
	"errors"
	"testing"
	"time"
)

func TestStoreCreatesAndFindsLatestReport(t *testing.T) {
	store := NewStore()
	store.now = func() time.Time {
		return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	}

	if _, err := store.Create(validReport("older", "2026-09-29T09:00:00Z", false)); err != nil {
		t.Fatalf("create older report: %v", err)
	}
	if _, err := store.Create(validReport("newer", "2026-09-29T10:00:00Z", false)); err != nil {
		t.Fatalf("create newer report: %v", err)
	}

	latest, err := store.Latest("eth")
	if err != nil {
		t.Fatalf("latest report: %v", err)
	}
	if latest.Envelope.ReportID != "newer" {
		t.Fatalf("got latest report %q, want newer", latest.Envelope.ReportID)
	}

	signals, err := store.LatestSignals("ETH", "1d")
	if err != nil {
		t.Fatalf("latest signals: %v", err)
	}
	if len(signals) != 1 || signals[0].Direction != "bearish" {
		t.Fatalf("unexpected signals: %#v", signals)
	}
}

func TestStoreRejectsDuplicateAndExecutablePosture(t *testing.T) {
	store := NewStore()
	if _, err := store.Create(validReport("same", "2026-09-29T10:00:00Z", false)); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if _, err := store.Create(validReport("same", "2026-09-29T10:00:00Z", false)); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate error = %v, want ErrConflict", err)
	}
	if _, err := store.Create(validReport("unsafe", "2026-09-29T10:00:00Z", true)); err == nil {
		t.Fatal("expected executable posture to be rejected")
	}
}

func validReport(id, asOf string, eligible bool) []byte {
	return []byte(`{
		"schema_version":"macro-risk.v1",
		"report_id":"` + id + `",
		"generated_at":"2026-09-29T10:05:00Z",
		"as_of":"` + asOf + `",
		"window":{
			"start":"2026-09-28T10:00:00Z",
			"end":"` + asOf + `"
		},
		"scope":{"assets":["BTC","ETH"],"horizons":["1d"]},
		"defaults_applied":[],
		"data_quality":{
			"status":"complete",
			"coverage_ratio":1,
			"freshness_seconds":300,
			"source_counts":{"tier_a":0,"tier_b":0,"tier_c":0,"tier_d":0},
			"limitations":[]
		},
		"sources":[],
		"events":[],
		"market_regime":{
			"risk_state":"unclear",
			"liquidity":"unclear",
			"volatility":"unclear",
			"confidence":null,
			"rationale_event_ids":[]
		},
		"signals":[{
			"id":"eth-1d",
			"asset":"ETH",
			"horizon":"1d",
			"direction":"bearish",
			"score":-42,
			"confidence":0.8,
			"category_weights":{"monetary_policy":1},
			"event_ids":[],
			"rationale":"test signal",
			"invalidation_conditions":[]
		}],
		"position_risks":[{
			"position_ref":"position-1",
			"asset":"ETH",
			"side":"unknown",
			"assessment_status":"incomplete",
			"posture":"review",
			"urgency":"low",
			"eligible":` + boolString(eligible) + `,
			"requires_human_approval":true,
			"confidence":null,
			"rationale":"position is not available",
			"trigger_conditions":[],
			"invalidation_conditions":[],
			"signal_ids":["eth-1d"],
			"event_ids":[],
			"missing_inputs":["position"],
			"next_review_at":null
		}],
		"contradictions":[],
		"summary":{"facts":[],"inferences":[],"decision":"review"}
	}`)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
