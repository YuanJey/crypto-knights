# Macro Risk Model

Use this model to make scoring reproducible and uncertainty visible. Scores are
decision support, not forecasts of return.

## Event Taxonomy

Assign one primary category to each canonical event:

- `monetary_policy`: Policy rates, central-bank guidance, balance sheets, and
  reserve conditions.
- `inflation_growth_labor`: Inflation, activity, employment, and productivity.
- `liquidity_credit`: Dollar liquidity, funding stress, sovereign debt,
  banking conditions, and credit events.
- `fiscal_policy`: Spending, taxation, debt issuance, and fiscal disputes.
- `regulation_legal`: Laws, enforcement, court decisions, sanctions, and market
  access.
- `geopolitics_security`: Conflict, trade restrictions, elections, diplomatic
  escalation, and critical infrastructure disruption.
- `crypto_systemic`: Stablecoins, custody, exchanges, protocols, market
  structure, and major forced-flow events.

Use secondary tags when an event spans categories. Aggregate it only under the
primary category to prevent double counting.

## Transmission Paths

Map each directional inference through at least one observable path:

- Real yields and policy-rate expectations
- Dollar strength and cross-border funding
- System liquidity and collateral availability
- Credit spreads and counterparty risk
- Energy, commodity, and inflation expectations
- Regulatory access and capital formation
- Spot, futures, options, ETF, and stablecoin flows
- Volatility, correlation, and liquidation cascades
- Exchange, custodian, bridge, or protocol availability

State the intermediate variable. A bare assertion such as "war is bearish for
BTC" is not a valid transmission analysis.

## Event Score

Score each asset and horizon separately after evidence deduplication.

Inputs:

- `direction`: `-1` bearish, `0` mixed or neutral, `1` bullish.
- `severity`: Integer from `1` immaterial to `5` systemic.
- `relevance`: Decimal from `0` unrelated to `1` directly relevant to the
  asset and horizon.
- `confidence`: Decimal from `0` unsupported to `1` strongly corroborated.
- `freshness`: Decimal from `0` outside the decision horizon to `1` current and
  effective within it.

Calculate:

```text
event_score = round(100 * direction * (severity / 5)
                    * relevance * confidence * freshness)
```

Keep the score in `[-100, 100]`. A direction of `0` means evidence supports a
mixed or neutral effect. Missing evidence is `null`, not zero.

Use these confidence anchors as defaults:

| Evidence state | Confidence range |
| --- | --- |
| Confirmed by authoritative primary data and independently corroborated | 0.85-1.00 |
| Confirmed by a primary source or two independent Tier B sources | 0.70-0.89 |
| Developing event with credible but incomplete confirmation | 0.50-0.74 |
| Disputed interpretation or only specialist support | 0.25-0.59 |
| Rumor or unverified claim | 0.00-0.24 |

Do not raise confidence because multiple articles repeat the same origin.

## Aggregation

Aggregate in this order:

1. Cluster duplicate reports into canonical events.
2. Calculate one event score per asset and horizon.
3. Calculate each category score as the confidence-weighted mean of its event
   scores.
4. Calculate the asset-horizon signal as the weighted mean of category scores.
5. Clamp the final signal to `[-100, 100]` and retain the contributing event
   IDs.

Category weights must come from caller configuration when provided. Otherwise
use equal weights and declare that default. Do not change weights after viewing
the outcome.

For a set of scores `s_i` and weights `w_i`:

```text
weighted_mean = sum(s_i * w_i) / sum(w_i)
```

Exclude `null` scores from both numerator and denominator. If no scored
evidence remains, the aggregate is `null`.

Map aggregate score to direction:

| Score | Direction |
| --- | --- |
| `-100` to `-20` | `bearish` |
| greater than `-20` and less than `20` | `neutral` |
| `20` to `100` | `bullish` |
| `null` | `unclear` |

Aggregate confidence must account for source quality, independent
corroboration, event agreement, freshness, and coverage. It must not exceed the
highest-confidence material driver when the drivers conflict.

## Market Regime

Report the following dimensions independently:

- `risk_state`: `risk_on`, `risk_off`, `mixed`, or `unclear`
- `liquidity`: `easing`, `tightening`, `neutral`, or `unclear`
- `volatility`: `expanding`, `contracting`, `stable`, or `unclear`

Do not derive all dimensions from price alone. Cite event or market-data IDs
for each regime label.

## Position Risk

Evaluate risk relative to the supplied position:

- A bearish asset signal increases risk for a long and may reduce risk for a
  short; apply the inverse for a bullish signal.
- Rising volatility and shrinking liquidity can increase both long and short
  risk through gaps, slippage, and liquidation.
- Scheduled binary events can justify hedging or reducing exposure even when
  directional conviction is low.
- Existing hedges, option convexity, cross-asset correlation, funding, and
  liquidation distance can materially change the posture.

Never calculate a hedge quantity without position size, contract multiplier,
instrument, and maximum hedge ratio. Never infer those inputs.

## Posture Policy

Choose one posture per supplied position:

| Posture | Meaning |
| --- | --- |
| `monitor` | Evidence is weak, stale, incomplete, or awaiting a named trigger |
| `hold` | Exposure remains within supplied limits and no risk threshold is breached |
| `reduce` | Directional risk is adverse and exposure should be lowered if the stated trigger occurs |
| `hedge` | Tail, event, or volatility risk warrants an offset within supplied hedge constraints |
| `close` | Supplied hard risk limits are breached or a caller-defined exit condition is met |
| `review` | Required position or mandate data is missing, contradictory, or invalid |

Default posture thresholds may guide analysis but cannot authorize execution:

- Use `monitor` when aggregate confidence is below `0.55`.
- Consider `reduce` when the exposure-adjusted adverse score is at least `35`
  and confidence is at least `0.65`.
- Consider `hedge` when the exposure-adjusted adverse score is at least `50`
  with confidence at least `0.70`, or when a scheduled binary event creates
  material gap risk.
- Use `close` only when a supplied hard limit or explicit caller exit condition
  is met. A macro score alone is insufficient.
- Use `review` when required position inputs are absent or inconsistent.

Every posture must include:

- Human-readable rationale
- Trigger conditions
- Invalidation conditions
- Urgency
- Confidence
- Evidence and signal IDs
- Next review time
- `eligible: false`
- `requires_human_approval: true`

The execution service must independently validate market conditions, account
state, risk limits, idempotency, and authorization. It must never treat this
report as an order instruction.
