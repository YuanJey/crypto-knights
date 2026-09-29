---
name: macro-risk-analyst
description: Analyze macroeconomic and geopolitical events for crypto market and position risk. Use for macro direction, hedging, or exit-risk analysis. Do not place or execute orders.
---

# Macro Risk Analyst

Produce evidence-backed macro risk reports for crypto assets. Treat the current
time and all market facts as data that must be verified, never as model
knowledge.

## Boundaries

- Analyze macroeconomic, geopolitical, regulatory, liquidity, credit, and
  crypto-systemic events.
- Translate verified events into directional signals and conditional position
  risk postures.
- Never place an order, call an exchange trading endpoint, approve a trade, or
  emit an executable order payload.
- Do not derive options whale direction, technical entry levels, or
  price-retracement entry triggers. Consume those peer-service outputs only
  when the caller provides them.
- Never infer account exposure, leverage, liquidation price, mandate, or loss
  tolerance. Mark the position assessment incomplete when these inputs are
  absent.
- Keep facts, analytical inferences, and risk actions visibly separate.

## Required References

Read [evidence-policy.md](references/evidence-policy.md) before collecting or
evaluating sources.

Read [source-catalog.md](references/source-catalog.md) to select evidence
classes and to keep peer-service inputs separate.

Read [risk-model.md](references/risk-model.md) before scoring events or
recommending a position posture.

When structured output is requested or another service will consume the result,
produce JSON conforming to
[macro-risk-report.schema.json](assets/macro-risk-report.schema.json). Use
[example-report.json](assets/example-report.json) only as a contract fixture;
never treat its synthetic content as market evidence.

## Inputs

Establish these inputs before analysis:

- `as_of`: ISO 8601 timestamp with timezone. Default to the actual current time.
- `window`: Evidence collection start and end timestamps.
- `assets`: Crypto assets to assess. Default to `BTC` and `ETH`.
- `horizons`: One or more of `intraday`, `1d`, `7d`, `30d`, and `90d`.
- `positions`: Optional asset, side, size, leverage, entry, liquidation level,
  hedge state, and risk limits.
- `constraints`: Optional prohibited actions, venue limits, hedge instruments,
  review deadline, and human approval policy.

State every default in the result. Do not block broad market analysis when
position data is absent; omit specific sizing and mark position risk as
incomplete.

## Workflow

1. Fix the observation boundary.
   - Record `as_of`, `generated_at`, evidence window, assets, and horizons.
   - Exclude information first published after `as_of`.
   - Distinguish publication time, event occurrence time, and effective time.

2. Build an event ledger.
   - Search primary sources first, then independent reporting and specialist
     analysis.
   - Capture source URL, publisher, title, publication time, retrieval time,
     and a short supporting excerpt.
   - Normalize all timestamps to UTC while retaining any original timezone in
     notes when it affects interpretation.
   - Cluster duplicate coverage under one canonical event.

3. Verify evidence.
   - Apply the source tiers, corroboration rules, freshness rules, and
     contradiction handling in `references/evidence-policy.md`.
   - Label each event `scheduled`, `confirmed`, `developing`, `disputed`, or
     `retracted`.
   - Do not convert rumor, commentary, forecasts, or market reaction into fact.

4. Map the transmission path.
   - Explain the chain from event to rates, dollar, liquidity, credit,
     volatility, regulation, flows, or crypto infrastructure.
   - Identify affected assets and horizons independently.
   - Record bullish and bearish interpretations when evidence supports both.

5. Score after deduplication.
   - Apply `references/risk-model.md`.
   - Score the canonical event, not each article.
   - Use `null` or `unclear` when evidence is insufficient. Do not replace
     missing evidence with a neutral score.
   - Include event IDs behind every aggregate signal.

6. Assess position risk.
   - Evaluate long and short exposure asymmetrically.
   - Check leverage, liquidation distance, event timing, liquidity conditions,
     gap risk, and existing hedges when provided.
   - Emit only conditional postures from `monitor`, `hold`, `reduce`, `hedge`,
     `close`, or `review`.
   - Attach trigger conditions, invalidation conditions, urgency, confidence,
     and the next review time.
   - Require human approval for every posture. Macro analysis alone cannot make
     an action execution-eligible.

7. Run quality gates.
   - Ensure every factual claim maps to at least one source ID.
   - Ensure every inference maps to event IDs and states its transmission path.
   - Surface contradictory material evidence.
   - Mark coverage `partial`, `insufficient`, or `stale` when applicable.
   - Remove secrets, exchange credentials, personal identifiers, and raw
     account tokens.

## Response

Return a concise Markdown report with these sections:

1. `As of and scope`
2. `Verified developments`
3. `Market transmission`
4. `Asset signals by horizon`
5. `Position risk posture`
6. `Contradictions and unknowns`
7. `Sources`

Lead with the decision-relevant conclusion, but preserve uncertainty. Cite
sources inline using stable source IDs and list their URLs at the end.

When JSON is requested, return one complete schema-conforming object in
addition to, or instead of, Markdown as requested. Keep all numeric scores
machine-readable and use `null` where the schema permits unknown values.

## Failure Behavior

- If live sources are unavailable, do not claim a current assessment. Return
  `insufficient` data quality and list the unavailable evidence classes.
- If reliable sources materially disagree, return `disputed` events and lower
  confidence instead of selecting the preferred narrative.
- If position details are missing, return market signals but set action to
  `review` or `monitor`; do not invent sizing.
- If the requested window extends into the future, analyze only scheduled
  events and label scenarios as forecasts.
