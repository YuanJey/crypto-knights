# Execution Service

Validates trigger requests, enforces risk policy, and places idempotent orders
through an exchange adapter.

Only the `paper` adapter is included. Any other `EXCHANGE_MODE` makes the
service refuse startup so a configuration error cannot silently place live
orders.

## Run

```bash
go run ./cmd/server
```

Configuration:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8084` |
| `EXCHANGE_MODE` | `paper` |
| `ALLOWED_SYMBOLS` | `BTCUSDT,ETHUSDT` |
| `MAX_ORDER_NOTIONAL` | `10000` |
| `MAX_TRIGGER_AGE_SECONDS` | `30` |

## API

- `POST /v1/executions` validates and submits an execution request.
- `GET /v1/executions` lists executions.
- `GET /v1/executions/{id}` returns an execution.
- `GET /healthz` reports process health.

An idempotency key is permanently bound to the complete normalized request.
Reusing it with changed fields returns `409 Conflict`. The maximum notional is
calculated from `quantity * reference_price`; a production exchange adapter
must additionally enforce current-price and account-level controls.
Persistence is currently in memory.
