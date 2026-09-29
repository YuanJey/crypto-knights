# Evidence Policy

Use this policy for every current-event claim in a macro risk report.

## Source Tiers

Assign one tier to each source:

| Tier | Source class | Examples | Permitted use |
| --- | --- | --- | --- |
| A | Primary and authoritative | Central banks, statistics agencies, regulators, courts, legislatures, official exchange status pages, issuer filings, protocol governance records, raw market or on-chain data | Establish what the source itself announced or recorded |
| B | Independent high-accountability reporting | Major news wires and established financial publications with named reporting | Corroborate events and provide independently reported context |
| C | Specialist analysis | Research firms, market analysts, industry publications, dashboards with disclosed methods | Supply interpretation or domain context, not sole proof of a high-impact event |
| D | Unverified or interested-party claims | Anonymous posts, social media, reposts, promotional material, unsourced screenshots | Discovery only unless the report is explicitly analyzing rumor propagation |

Do not infer truth from search-result snippets. Open the source and verify the
claim, date, and context.

## Corroboration

- Accept a Tier A source as evidence of its own announcement, ruling, release,
  or recorded data.
- Require either one Tier A source or two independent Tier B sources for a
  high-impact event.
- Require at least one independent source when a company, protocol, exchange,
  government, or campaign makes a claim about another party.
- Treat syndicated copies, articles citing the same unnamed source, and posts
  repeating one origin as one source.
- Use Tier C to support an inference only when its method and underlying data
  are visible.
- Never promote Tier D into a confirmed fact by repetition count.

## Time Semantics

Capture the following timestamps separately:

- `published_at`: When the source published the information.
- `occurred_at`: When the event happened, if known.
- `retrieved_at`: When the source was inspected.
- `effective_window`: When the event is expected to affect the assessed market
  horizon.

Normalize machine-readable timestamps to UTC using ISO 8601. Do not use a
source first published after the report's `as_of` boundary. When a source is
updated, use the update only if the updated timestamp is at or before `as_of`.

An old article is not fresh evidence merely because it was reposted. Scheduled
events must remain `scheduled` until a source confirms occurrence.

## Event Identity and Deduplication

Create one canonical event for all coverage of the same underlying occurrence.
Build its `canonical_key` from stable facts:

```text
lowercase(category | principal_actor | action | target | occurred_date)
```

Normalize whitespace and names before comparison. Do not include publisher,
headline wording, URL parameters, or retrieval time in the key.

Merge records when they describe the same actor, action, target, and occurrence
window. Keep separate events when:

- A later action changes legal or operational state.
- An announced plan becomes an enacted decision.
- A scheduled release and its actual value are analytically distinct.
- Independent incidents have different affected systems or effective windows.

Score the canonical event once. Retain all independent supporting and
contradicting sources under that event.

## Facts and Inferences

Write facts as narrow claims that a cited source directly supports. Write
inferences as explicit causal statements:

```text
Fact: [verified event or value] [source IDs]
Inference: [event] may affect [asset and horizon] through [transmission path].
```

Do not use market price movement as proof of a narrative. Record the movement
as a separate market-data fact and state causal attribution as an inference.

## Contradictions and Corrections

- Preserve material disagreements instead of averaging them away.
- Identify whether disagreement concerns facts, timing, magnitude, or
  interpretation.
- Lower event confidence when independent credible sources conflict.
- Mark an event `retracted` when its authoritative origin retracts it.
- Never silently replace an earlier claim. Include the correction and explain
  which downstream signal changed.

## Data Quality

Set report status using these rules:

- `complete`: Required evidence classes were checked, critical events meet
  corroboration requirements, and freshness fits every requested horizon.
- `partial`: Some evidence classes or regions are unavailable, but the
  supported conclusions remain useful.
- `insufficient`: Critical claims cannot be verified or coverage is too sparse
  for a directional conclusion.
- `stale`: Available evidence predates the decision horizon or exceeds the
  caller's freshness limit.

Report a coverage ratio only when the expected evidence classes were declared
before collection. List missing classes and access failures as limitations.

## Minimum Source Record

For every cited source, retain:

- Stable source ID
- URL
- Publisher
- Title
- Tier and source type
- Publication and retrieval timestamps
- Short excerpt or exact data point supporting the claim
- The claim or event ID it supports or contradicts

Never store authentication headers, cookies, API keys, complete paywalled
articles, or unrelated personal data.
