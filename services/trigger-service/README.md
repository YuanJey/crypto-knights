# Trigger Service

Maintains price-trigger state machines and dispatches eligible entries to the
execution service.

For a long rule:

1. Activate when price is less than or equal to `activation_price`.
2. Track the lowest price through the configured window.
3. Trigger when price rebounds by `retracement` from that minimum.

A short rule is symmetric and tracks the highest price before falling by the
configured retracement.

## Run

Start the execution service first, then:

```bash
go run ./cmd/server
```

Configuration:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8083` |
| `EXECUTION_SERVICE_URL` | `http://localhost:8084` |

## API

- `POST /v1/rules` creates a rule.
- `GET /v1/rules?symbol=ETHUSDT` lists rules.
- `GET /v1/rules/{id}` returns a rule and its current state.
- `POST /v1/rules/{id}/cancel` cancels an active rule.
- `POST /v1/rules/{id}/retry` retries a failed execution dispatch.
- `POST /v1/ticks` applies one timestamped market tick.
- `GET /healthz` reports process health.

Ticks with a timestamp not newer than a rule's last observed tick are ignored.
Each rule uses `trigger:{rule_id}` as a stable execution idempotency key.
Persistence and durable dispatch retries are currently in memory.
