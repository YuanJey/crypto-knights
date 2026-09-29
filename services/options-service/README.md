# Options Service

Ingests normalized option trades and aggregates large-premium directional
flow. It treats aggressive call buys and put sells as bullish premium, and put
buys and call sells as bearish premium.

## Run

```bash
go run ./cmd/server
```

Configuration:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8082` |
| `MIN_PREMIUM_NOTIONAL` | `100000` |
| `MAX_KEY_LEVELS` | `10` |

Premium notional is:

```text
contracts * premium_usd * contract_multiplier
```

## API

- `POST /v1/trades` ingests one normalized trade.
- `GET /v1/trades/{id}` returns an ingested trade.
- `GET /v1/signals?underlying=ETH&window_seconds=3600` aggregates flow.
- `GET /healthz` reports process health.

`premium_usd` must be normalized to USD by the source adapter before ingestion.
Signals include the strongest expiry/strike clusters in `key_levels`. All
prices, quantities, and notionals use exact decimal JSON strings. Source
exchange adapters are intentionally separate from this normalization API.
Persistence is currently in memory.
