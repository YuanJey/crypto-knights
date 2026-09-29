# Source Catalog

Use this catalog as the default coverage matrix, not as a fixed allowlist.
Prefer machine-readable primary releases where available. Discover and verify
the current official URL at collection time; do not rely on a remembered URL.

## Scheduled Macroeconomic Evidence

Cover the jurisdictions material to the requested assets and positions:

| Evidence class | Primary-source examples | Key facts |
| --- | --- | --- |
| Monetary policy | Federal Reserve, ECB, Bank of England, Bank of Japan, People's Bank of China, Swiss National Bank | Decisions, minutes, speeches, balance sheets, liquidity facilities |
| Inflation | BLS, BEA, Eurostat, national statistics agencies | CPI, PCE, PPI, revisions, methodology changes |
| Growth | BEA, Eurostat, national statistics agencies | GDP, industrial production, retail activity, revisions |
| Labor | BLS, national labor ministries and statistics agencies | Payrolls, unemployment, claims, wages, revisions |
| Fiscal and sovereign funding | Finance ministries, US Treasury, debt-management offices | Issuance calendars, auctions, balances, spending, tax changes |
| Energy and commodities | EIA, IEA, OPEC publications, national energy agencies | Inventories, production, disruptions, policy changes |

Read the release itself and its tables. Record consensus estimates only as
separate context with their own source.

## Liquidity and Credit Evidence

Check primary or transparent data for:

- Central-bank assets, reserve balances, and liquidity facilities
- Sovereign yield curves, real yields, and auction results
- Dollar index or a declared dollar proxy
- Funding spreads and money-market stress
- Investment-grade and high-yield credit spreads
- Stablecoin supply, redemptions, and reserve attestations
- Material exchange, custodian, or settlement outages

State the data timestamp and market close convention. Do not combine daily,
intraday, and delayed series without labeling the mismatch.

## Geopolitical and Security Evidence

Prioritize:

1. Official statements, legal texts, sanctions lists, maritime or aviation
   notices, and emergency authorities.
2. Independent news wires with reporters or named officials.
3. Established regional reporting with disclosed sourcing.
4. Specialist conflict, shipping, energy, or supply-chain analysis with
   visible methods.

Track the concrete market channel, such as sanctions scope, energy supply,
shipping capacity, payment access, cyber availability, or capital controls.
Political rhetoric without an operational change may still affect volatility,
but must not be scored as enacted policy.

## Regulation and Legal Evidence

Use regulator releases, court dockets or opinions, legislation, official
gazettes, and issuer filings as primary evidence. Separate:

- Proposal from enacted rule
- Filing from approval
- Complaint from judgment
- Guidance from binding law
- Investigation from enforcement outcome
- Effective date from publication date

For crypto-specific policy, assess access, custody, listing, stablecoin,
staking, mining, taxation, and capital-flow channels independently.

## Crypto-Systemic Evidence

Use first-party status pages, incident reports, governance records, reserve
disclosures, and independently verifiable on-chain or market data. Check:

- Stablecoin depegs, issuance, redemption, and reserve concerns
- Exchange, custodian, bridge, oracle, and settlement incidents
- Protocol governance or emergency actions
- ETF or fund creations, redemptions, and disclosed holdings
- Forced liquidations and broad market dislocations

Treat exchange statements about solvency or reserves as interested-party
claims unless independently verifiable.

## Peer-Service Inputs

The following data belongs to peer services and is optional context:

- Options positioning, volatility surfaces, block trades, and large-flow
  direction from the options service
- Technical entry levels and trigger definitions from the strategy service
- Live position, leverage, orders, and exchange state from the execution
  service

Label every peer input with its producer, schema version, and observation time.
Do not recreate another service's signal when its authoritative output is
available. Never let a peer signal replace source verification for a macro
event.

## Collection Priorities

For each requested horizon, collect in this order:

1. Releases or events occurring inside the evidence window
2. Scheduled events inside the decision horizon
3. Corrections, revisions, retractions, or effective-date changes
4. Market data that confirms or contradicts the proposed transmission path
5. Background analysis needed to interpret material events

Stop expanding coverage when additional sources repeat the same origin without
adding independent confirmation, a contradiction, or a new transmission
channel.

Respect access controls, licensing, robots policies, and rate limits. Do not
bypass paywalls or authentication. When a critical source cannot be inspected,
record the failure and lower data quality.
